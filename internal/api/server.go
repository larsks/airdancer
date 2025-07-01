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
	"github.com/go-chi/cors"
	"github.com/larsks/airdancer/internal/gpio"
	"github.com/larsks/airdancer/internal/piface"
	"github.com/larsks/airdancer/internal/switchcollection"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Server represents the API server.

type Server struct {
	listenAddr string
	switches   switchcollection.SwitchCollection
	mutex      sync.Mutex
	timers     map[string]*time.Timer
	router     *chi.Mux
}

// Config holds the configuration for the API server.

type (
	PiFaceConfig struct {
		SPIDev string `mapstructure:"spidev"`
	}

	GPIOConfig struct {
		Pins []string `mapstructure:"pins"`
	}

	DummyConfig struct {
		SwitchCount uint `mapstructure:"switch_count"`
	}

	Config struct {
		ListenAddress string `mapstructure:"listen-address"`
		ListenPort    int    `mapstructure:"listen-port"`
		ConfigFile    string `mapstructure:"config-file"`
		Driver        string `mapstructure:"driver"`
		GPIOConfig    GPIOConfig
		PiFaceConfig  PiFaceConfig
		DummyConfig   DummyConfig
	}
)

// NewConfig creates a new Config instance with default values.

func NewConfig() *Config {
	return &Config{
		ListenAddress: "",
		ListenPort:    8080,
		Driver:        "piface",
		PiFaceConfig: PiFaceConfig{
			SPIDev: "/dev/spidev0.0",
		},
		DummyConfig: DummyConfig{
			SwitchCount: 4,
		},
	}
}

// AddFlags adds pflag flags for the configuration.

func (c *Config) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ConfigFile, "config", "", "Config file to use")
	fs.StringVar(&c.ListenAddress, "listen-address", c.ListenAddress, "Listen address for http server")
	fs.IntVar(&c.ListenPort, "listen-port", c.ListenPort, "Listen port for http server")
	fs.StringVar(&c.Driver, "driver", c.Driver, "Driver to use (piface, gpio, or dummy)")
	fs.StringVar(&c.PiFaceConfig.SPIDev, "piface.spidev", c.PiFaceConfig.SPIDev, "SPI device to use")
	fs.StringSliceVar(&c.GPIOConfig.Pins, "gpio.pins", c.GPIOConfig.Pins, "GPIO pins to use (for gpio driver)")
	fs.UintVar(&c.DummyConfig.SwitchCount, "dummy.switch-count", c.DummyConfig.SwitchCount, "Number of switches for dummy driver")
}

// LoadConfig loads the configuration from a file and binds it to the Config struct.

func (c *Config) LoadConfig() error {
	v := viper.New()
	v.SetDefault("listen-address", c.ListenAddress)
	v.SetDefault("listen-port", c.ListenPort)
	v.SetDefault("driver", c.Driver)
	v.SetDefault("piface.spidev", c.PiFaceConfig.SPIDev)
	v.SetDefault("gpio.pins", c.GPIOConfig.Pins)
	v.SetDefault("dummy.switch_count", c.DummyConfig.SwitchCount)

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
	var switches switchcollection.SwitchCollection
	var err error

	switch cfg.Driver {
	case "piface":
		switches, err = piface.NewPiFace(true, cfg.PiFaceConfig.SPIDev)
		if err != nil {
			return nil, fmt.Errorf("failed to open PiFace: %w", err)
		}
	case "gpio":
		switches, err = gpio.NewGPIOSwitchCollection(true, cfg.GPIOConfig.Pins)
		if err != nil {
			return nil, fmt.Errorf("failed to create gpio driver: %w", err)
		}
	case "dummy":
		switches = switchcollection.NewDummySwitchCollection(cfg.DummyConfig.SwitchCount)
	default:
		return nil, fmt.Errorf("unknown driver: %s", cfg.Driver)
	}

	if err := switches.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize driver: %w", err)
	}

	s := &Server{
		listenAddr: fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort),
		switches:   switches,
		timers:     make(map[string]*time.Timer),
		router:     chi.NewRouter(),
	}

	s.router.Use(middleware.Logger)
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://*", "https://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	// Set up routes with validation middleware
	s.router.Route("/switch", func(r chi.Router) {
		// GET endpoints for status queries
		r.With(s.validateSwitchID, s.validateSwitchExists).Get("/{id}", s.switchStatusHandler)

		// POST endpoints for switch control
		r.With(
			s.validateSwitchID,
			s.validateSwitchExists,
			s.validateJSONRequest,
			s.validateSwitchRequest,
		).Post("/{id}", s.switchHandler)
	})

	return s, nil
}

// Start starts the API server.

func (s *Server) Start() error {
	// Initialize all switches to off
	if err := s.switches.TurnOff(); err != nil {
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
	defer cancel() //nolint:errcheck

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	log.Println("Server gracefully stopped")
	return nil
}

// Close closes the PiFace connection.

func (s *Server) Close() error {
	return s.switches.Close()
}
