package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/larsks/airdancer/internal/piface"
	"github.com/larsks/airdancer/internal/switchdriver"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Server represents the API server.

type Server struct {
	listenAddr  string
	switches    switchdriver.SwitchCollection
	outputState uint8
	mutex       sync.Mutex
	timers      map[uint]*time.Timer
	router      *chi.Mux
}

// Config holds the configuration for the API server.

type Config struct {
	ListenAddress string `mapstructure:"listen-address"`
	ListenPort    int    `mapstructure:"listen-port"`
	SPIDev        string `mapstructure:"spidev"`
	ConfigFile    string `mapstructure:"config-file"`
}

// NewConfig creates a new Config instance with default values.

func NewConfig() *Config {
	return &Config{
		ListenAddress: "",
		ListenPort:    8080,
		SPIDev:        "/dev/spidev0.0",
	}
}

// AddFlags adds pflag flags for the configuration.

func (c *Config) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ConfigFile, "config-file", "", "Config file to use")
	fs.StringVar(&c.SPIDev, "spidev", c.SPIDev, "SPI device to use")
	fs.StringVar(&c.ListenAddress, "listen-address", c.ListenAddress, "Listen address for http server")
	fs.IntVar(&c.ListenPort, "listen-port", c.ListenPort, "Listen port for http server")
}

// LoadConfig loads the configuration from a file and binds it to the Config struct.

func (c *Config) LoadConfig() error {
	v := viper.New()
	v.SetDefault("listen-address", c.ListenAddress)
	v.SetDefault("listen-port", c.ListenPort)
	v.SetDefault("spidev", c.SPIDev)

	if c.ConfigFile != "" {
		v.SetConfigFile(c.ConfigFile)
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	if err := v.BindPFlags(pflag.CommandLine); err != nil {
		return fmt.Errorf("failed to bind flags to config options: %w", err)
	}

	if err := v.Unmarshal(c); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// NewServer creates a new Server instance.

func NewServer(cfg *Config) (*Server, error) {
	pf, err := piface.NewPiFace(cfg.SPIDev)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PiFace: %w", err)
	}

	s := &Server{
		listenAddr: fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort),
		switches:   pf,
		timers:     make(map[uint]*time.Timer),
		router:     chi.NewRouter(),
	}

	s.router.Use(middleware.Logger)
	s.router.Post("/switch/{id}", s.switchHandler)
	s.router.Get("/switch/{id}", s.switchStatusHandler)

	return s, nil
}

// Start starts the API server.

func (s *Server) Start() error {
	// Initialize all switches to off
	if err := s.switches.TurnAllOff(); err != nil {
		return fmt.Errorf("failed to initialize switches: %w", err)
	}

	srv := &http.Server{
		Addr:    s.listenAddr,
		Handler: s.router,
	}

	go func() {
		log.Printf("Starting server on %s", s.listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	log.Println("Server gracefully stopped")
	return nil
}

// Close closes the PiFace connection.

func (s *Server) Close() {
	s.switches.Close()
}
