package ui

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/larsks/airdancer/internal/config"
	"github.com/larsks/airdancer/internal/static"
	"github.com/spf13/pflag"
)

// Config holds the configuration for the UI server.
type Config struct {
	ListenAddress string `mapstructure:"listen-address"`
	ListenPort    int    `mapstructure:"listen-port"`
	ConfigFile    string `mapstructure:"config-file"`
	APIBaseURL    string `mapstructure:"api-base-url"`
}

// NewConfig creates a new Config instance with default values.
func NewConfig() *Config {
	return &Config{
		ListenAddress: "",
		ListenPort:    8081,
		APIBaseURL:    "http://localhost:8080",
	}
}

// AddFlags adds pflag flags for the configuration.
func (c *Config) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ConfigFile, "config", "", "Config file to use")
	fs.StringVar(&c.ListenAddress, "listen-address", c.ListenAddress, "Listen address for UI server")
	fs.IntVar(&c.ListenPort, "listen-port", c.ListenPort, "Listen port for UI server")
	fs.StringVar(&c.APIBaseURL, "api-base-url", c.APIBaseURL, "Base URL for the API server")
}

// LoadConfig loads the configuration from a file and binds it to the Config struct.
func (c *Config) LoadConfig() error {
	return c.LoadConfigWithFlagSet(pflag.CommandLine)
}

// LoadConfigWithFlagSet loads the configuration using a custom flag set
func (c *Config) LoadConfigWithFlagSet(fs *pflag.FlagSet) error {
	loader := config.NewConfigLoader()
	loader.SetConfigFile(c.ConfigFile)

	// Set default values
	loader.SetDefaults(map[string]any{
		"listen-address": "",
		"listen-port":    8081,
		"api-base-url":   "http://localhost:8080",
	})

	return loader.LoadConfigWithFlagSet(c, fs)
}

type UIServer struct {
	listenAddr string
	apiBaseURL string
	router     *chi.Mux
}

// NewUIServer creates a new UI server instance.
func NewUIServer(cfg *Config) *UIServer {
	ui := &UIServer{
		listenAddr: fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort),
		apiBaseURL: cfg.APIBaseURL,
		router:     chi.NewRouter(),
	}

	ui.setupRoutes()
	return ui
}

func (ui *UIServer) setupRoutes() {
	ui.router.Use(middleware.Logger)
	ui.router.Use(middleware.Recoverer)

	// Serve the main page at root
	ui.router.Get("/", ui.indexHandler)

	// Serve static assets from the shared static package
	ui.router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(static.GetAssets()))))
}

func (ui *UIServer) indexHandler(w http.ResponseWriter, r *http.Request) {
	// Get switch controller JavaScript
	switchControllerJS, err := static.GetSwitchControllerJS()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get switch controller JS: %v", err), http.StatusInternalServerError)
		return
	}

	// Get switch control content HTML
	contentHTML, err := static.GetSwitchControlContent()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get switch control content: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := static.TemplateData{
		Title:         "Airdancer Switch Control",
		DefaultStatus: "Connecting...",
		Content:       template.HTML(contentHTML),
		ExtraJS: template.HTML(fmt.Sprintf(`
			<script>
				%s
				
				// Initialize the switch controller when the page loads
				document.addEventListener('DOMContentLoaded', () => {
					new SwitchController('%s');
				});
			</script>
		`, switchControllerJS, ui.apiBaseURL)),
	}

	// Render the template
	html, err := static.RenderTemplate(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to render template: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html)) //nolint:errcheck
}

func (ui *UIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ui.router.ServeHTTP(w, r)
}

func (ui *UIServer) Handler() http.Handler {
	return ui.router
}

// Start starts the UI server.
func (ui *UIServer) Start() error {
	srv := &http.Server{
		Addr:    ui.listenAddr,
		Handler: ui.router,
	}

	go func() {
		log.Printf("starting UI server on %s", ui.listenAddr)
		log.Printf("API URL: %s", ui.apiBaseURL)
		log.Printf("open http://localhost%s in your browser", ui.listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("UI server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down UI server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrServerShutdownFailed, err)
	}

	log.Println("UI server gracefully stopped")
	return nil
}
