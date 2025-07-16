package ui

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/larsks/airdancer/internal/config"
	"github.com/spf13/pflag"
)

//go:embed static/*
var staticFiles embed.FS

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
	loader := config.NewConfigLoader()
	loader.SetConfigFile(c.ConfigFile)

	// Set default values
	loader.SetDefaults(map[string]any{
		"listen-address": "",
		"listen-port":    8081,
		"api-base-url":   "http://localhost:8080",
	})

	return loader.LoadConfig(c)
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

	// Serve static files from embedded filesystem
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("failed to create static filesystem: %v", err))
	}

	// Serve the main page at root
	ui.router.Get("/", ui.indexHandler)

	// Serve static assets
	ui.router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
}

func (ui *UIServer) indexHandler(w http.ResponseWriter, r *http.Request) {
	// Read the index.html file from embedded filesystem
	indexFile, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read index.html: %v", err), http.StatusInternalServerError)
		return
	}

	// Replace the API_BASE_URL placeholder with the actual API URL
	indexContent := string(indexFile)
	indexContent = strings.ReplaceAll(indexContent, "{{API_BASE_URL}}", ui.apiBaseURL)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(indexContent)) //nolint:errcheck
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
	defer cancel() //nolint:errcheck

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrServerShutdownFailed, err)
	}

	log.Println("UI server gracefully stopped")
	return nil
}
