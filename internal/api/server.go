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
	"github.com/larsks/airdancer/internal/blink"
	"github.com/larsks/airdancer/internal/config"
	"github.com/larsks/airdancer/internal/gpio"
	"github.com/larsks/airdancer/internal/piface"
	"github.com/larsks/airdancer/internal/switchcollection"
	"github.com/spf13/pflag"
)

// Server represents the API server.

type Server struct {
	listenAddr string
	switches   switchcollection.SwitchCollection
	mutex      sync.Mutex
	timers     map[string]*time.Timer
	blinkers   map[string]*blink.Blink
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
		ListenAddress string       `mapstructure:"listen-address"`
		ListenPort    int          `mapstructure:"listen-port"`
		ConfigFile    string       `mapstructure:"config-file"`
		Driver        string       `mapstructure:"driver"`
		GPIOConfig    GPIOConfig   `mapstructure:"gpio"`
		PiFaceConfig  PiFaceConfig `mapstructure:"piface"`
		DummyConfig   DummyConfig  `mapstructure:"dummy"`
	}
)

// NewConfig creates a new Config instance with default values.

func NewConfig() *Config {
	return &Config{
		ListenAddress: "",
		ListenPort:    8080,
		Driver:        "dummy",
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
	loader := config.NewConfigLoader()
	loader.SetConfigFile(c.ConfigFile)

	// Set default values
	loader.SetDefaults(map[string]any{
		"listen_address":     "",
		"listen_port":        8080,
		"driver":             "dummy",
		"piface.spidev":      "/dev/spidev0.0",
		"gpio.pins":          []string{},
		"dummy.switch_count": 4,
	})

	return loader.LoadConfig(c)
}

// NewServer creates a new Server instance.

func NewServer(cfg *Config) (*Server, error) {
	var switches switchcollection.SwitchCollection
	var err error

	switch cfg.Driver {
	case "piface":
		switches, err = piface.NewPiFace(true, cfg.PiFaceConfig.SPIDev)
		if err != nil {
			return nil, fmt.Errorf("%w on %s: %v", ErrPiFaceInitFailed, cfg.PiFaceConfig.SPIDev, err)
		}
	case "gpio":
		switches, err = gpio.NewGPIOSwitchCollection(true, cfg.GPIOConfig.Pins)
		if err != nil {
			return nil, fmt.Errorf("%w with pins %v: %v", ErrGPIOInitFailed, cfg.GPIOConfig.Pins, err)
		}
	case "dummy":
		switches = switchcollection.NewDummySwitchCollection(cfg.DummyConfig.SwitchCount)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownDriver, cfg.Driver)
	}

	if err := switches.Init(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDriverInitFailed, err)
	}

	listenAddr := fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort)
	return newServerWithSwitches(switches, listenAddr, true), nil
}

// newServerWithSwitches creates a new Server instance with the given switches.
// If addProductionMiddleware is true, adds logger and CORS middleware.
func newServerWithSwitches(switches switchcollection.SwitchCollection, listenAddr string, addProductionMiddleware bool) *Server {
	s := &Server{
		listenAddr: listenAddr,
		switches:   switches,
		timers:     make(map[string]*time.Timer),
		blinkers:   make(map[string]*blink.Blink),
		router:     chi.NewRouter(),
	}

	if addProductionMiddleware {
		s.router.Use(middleware.Logger)
		s.router.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"http://*", "https://*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           300, // Maximum value not ignored by any of major browsers
		}))
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures the HTTP routes and middleware for the server.
func (s *Server) setupRoutes() {
	s.router.Get("/", s.listRoutesHandler)

	// Set up routes with validation middleware
	s.router.Route("/switch", func(r chi.Router) {
		// GET endpoints for status queries - only need basic ID validation for status
		r.With(
			s.validateJSONRequest,
			s.validateSwitchID,
			s.validateSwitchExists,
		).Get("/{id}", s.switchStatusHandler)

		// GET endpoint for blink status - only need basic ID validation for status
		r.With(
			s.validateJSONRequest,
			s.validateSwitchID,
			s.validateSwitchExists,
		).Get("/{id}/blink", s.blinkStatusHandler)

		// POST endpoints for switch control - restore full validation middleware chain
		r.With(
			s.validateJSONRequest,
			s.validateSwitchID,
			s.validateSwitchExists,
			s.validateSwitchRequest,
		).Post("/{id}", s.switchHandler)

		// POST endpoints for blink control
		r.With(
			s.validateJSONRequest,
			s.validateSwitchID,
			s.validateSwitchExists,
			s.validateBlinkRequest,
		).Post("/{id}/blink", s.blinkHandler)
	})
}

// Start starts the API server.

func (s *Server) Start() error {
	// Initialize all switches to off
	if err := s.switches.TurnOff(); err != nil {
		return fmt.Errorf("%w: %v", ErrSwitchInitFailed, err)
	}

	srv := &http.Server{
		Addr:    s.listenAddr,
		Handler: s.router,
	}

	go func() {
		log.Printf("starting server on %s", s.listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() //nolint:errcheck

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrServerShutdownFailed, err)
	}

	log.Println("server gracefully stopped")
	return nil
}

// Close closes the PiFace connection.

func (s *Server) Close() error {
	return s.switches.Close()
}

func (s *Server) ListRoutes() [][]string {
	routes := [][]string{}

	chi.Walk(s.router, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error { //nolint:errcheck
		routes = append(routes, []string{method, route})
		return nil
	})

	return routes
}
