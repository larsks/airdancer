package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/larsks/airdancer/internal/blink"
	"github.com/larsks/airdancer/internal/config"
	"github.com/larsks/airdancer/internal/flipflop"
	"github.com/larsks/airdancer/internal/piface"
	"github.com/larsks/airdancer/internal/switchcollection"
	"github.com/larsks/airdancer/internal/switchcollection/gpio"
	"github.com/spf13/pflag"
)

type timerData struct {
	timer    *time.Timer
	duration time.Duration
}

// ResolvedSwitch represents a switch that has been resolved to a specific collection and index.
type ResolvedSwitch struct {
	Name       string
	Collection switchcollection.SwitchCollection
	Index      uint
	Switch     switchcollection.Switch
}

// Server represents the API server.
type Server struct {
	listenAddr  string
	collections map[string]switchcollection.SwitchCollection
	switches    map[string]*ResolvedSwitch
	groups      map[string]*SwitchGroup
	mutex       sync.Mutex
	timers      map[string]*timerData
	blinkers    map[string]*blink.Blink
	flipflops   map[string]*flipflop.Flipflop
	router      *chi.Mux
}

// Config holds the configuration for the API server.

type (
	PiFaceDriverConfig struct {
		SPIDev      string `mapstructure:"spidev"`
		MaxSwitches uint   `mapstructure:"max-switches"`
	}

	GPIODriverConfig struct {
		Pins []string `mapstructure:"pins"`
	}

	DummyDriverConfig struct {
		SwitchCount uint `mapstructure:"switch-count"`
	}

	CollectionConfig struct {
		Driver       string                 `mapstructure:"driver"`
		DriverConfig map[string]interface{} `mapstructure:"driverconfig"`
	}

	SwitchConfig struct {
		Spec string `mapstructure:"spec"`
	}

	GroupConfig struct {
		Switches []string `mapstructure:"switches"`
	}

	Config struct {
		ListenAddress string                      `mapstructure:"listen-address"`
		ListenPort    int                         `mapstructure:"listen-port"`
		ConfigFile    string                      `mapstructure:"config-file"`
		Collections   map[string]CollectionConfig `mapstructure:"collections"`
		Switches      map[string]SwitchConfig     `mapstructure:"switches"`
		Groups        map[string]GroupConfig      `mapstructure:"groups"`
	}
)

// NewConfig creates a new Config instance with default values.

func NewConfig() *Config {
	return &Config{
		ListenAddress: "",
		ListenPort:    8080,
		Collections:   make(map[string]CollectionConfig),
		Switches:      make(map[string]SwitchConfig),
		Groups:        make(map[string]GroupConfig),
	}
}

// AddFlags adds pflag flags for the configuration.

func (c *Config) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ConfigFile, "config", "", "Config file to use")
	fs.StringVar(&c.ListenAddress, "listen-address", c.ListenAddress, "Listen address for http server")
	fs.IntVar(&c.ListenPort, "listen-port", c.ListenPort, "Listen port for http server")
}

// LoadConfig loads the configuration from a file and binds it to the Config struct.

func (c *Config) LoadConfig() error {
	loader := config.NewConfigLoader()
	loader.SetConfigFile(c.ConfigFile)

	// Set default values
	loader.SetDefaults(map[string]any{
		"listen-address": "",
		"listen-port":    8080,
		"collections":    make(map[string]CollectionConfig),
		"switches":       make(map[string]SwitchConfig),
		"groups":         make(map[string]GroupConfig),
	})

	return loader.LoadConfig(c)
}

// createSwitchCollection creates a switch collection based on the driver and config.
func createSwitchCollection(collectionName string, collectionCfg CollectionConfig) (switchcollection.SwitchCollection, error) {
	switch collectionCfg.Driver {
	case "piface":
		var pfCfg PiFaceDriverConfig
		if err := mapConfigToStruct(collectionCfg.DriverConfig, &pfCfg); err != nil {
			return nil, fmt.Errorf("failed to parse piface config for collection %s: %w", collectionName, err)
		}

		spidev := pfCfg.SPIDev
		if spidev == "" {
			spidev = "/dev/spidev0.0"
		}

		sc, err := piface.NewPiFace(true, spidev, pfCfg.MaxSwitches)
		if err != nil {
			return nil, fmt.Errorf("%w on %s for collection %s: %v", ErrPiFaceInitFailed, spidev, collectionName, err)
		}
		return sc, nil

	case "gpio":
		var gpioCfg GPIODriverConfig
		if err := mapConfigToStruct(collectionCfg.DriverConfig, &gpioCfg); err != nil {
			return nil, fmt.Errorf("failed to parse gpio config for collection %s: %w", collectionName, err)
		}

		sc, err := gpio.NewGPIOSwitchCollection(true, gpioCfg.Pins)
		if err != nil {
			return nil, fmt.Errorf("%w with pins %v for collection %s: %v", ErrGPIOInitFailed, gpioCfg.Pins, collectionName, err)
		}
		return sc, nil

	case "dummy":
		var dummyCfg DummyDriverConfig
		if err := mapConfigToStruct(collectionCfg.DriverConfig, &dummyCfg); err != nil {
			return nil, fmt.Errorf("failed to parse dummy config for collection %s: %w", collectionName, err)
		}

		if dummyCfg.SwitchCount == 0 {
			dummyCfg.SwitchCount = 4
		}

		return switchcollection.NewDummySwitchCollection(dummyCfg.SwitchCount), nil

	default:
		return nil, fmt.Errorf("%w: %s for collection %s", ErrUnknownDriver, collectionCfg.Driver, collectionName)
	}
}

// mapConfigToStruct converts a map[string]interface{} to a specific struct type.
func mapConfigToStruct(configMap map[string]interface{}, target interface{}) error {
	// This is a simple implementation. For a more robust solution, you might want to use
	// a library like mapstructure, but this handles the basic cases we need.
	switch t := target.(type) {
	case *PiFaceDriverConfig:
		if spidev, ok := configMap["spidev"].(string); ok {
			t.SPIDev = spidev
		}
		if maxSwitches, ok := configMap["max-switches"].(uint); ok {
			t.MaxSwitches = maxSwitches
		} else if maxSwitches, ok := configMap["max-switches"].(int); ok {
			t.MaxSwitches = uint(maxSwitches)
		}
	case *GPIODriverConfig:
		if pins, ok := configMap["pins"].([]interface{}); ok {
			t.Pins = make([]string, len(pins))
			for i, pin := range pins {
				if pinStr, ok := pin.(string); ok {
					t.Pins[i] = pinStr
				}
			}
		} else if pins, ok := configMap["pins"].([]string); ok {
			t.Pins = pins
		}
	case *DummyDriverConfig:
		if switchCount, ok := configMap["switch-count"].(uint); ok {
			t.SwitchCount = switchCount
		} else if switchCount, ok := configMap["switch-count"].(int); ok {
			t.SwitchCount = uint(switchCount)
		}
	}
	return nil
}

// NewServer creates a new Server instance.
func NewServer(cfg *Config) (*Server, error) {
	collections := make(map[string]switchcollection.SwitchCollection)
	switches := make(map[string]*ResolvedSwitch)
	groups := make(map[string]*SwitchGroup)

	// Create all switch collections
	for collectionName, collectionCfg := range cfg.Collections {
		if collectionName == "" {
			return nil, fmt.Errorf("collection name cannot be empty")
		}

		sc, err := createSwitchCollection(collectionName, collectionCfg)
		if err != nil {
			return nil, err
		}

		if err := sc.Init(); err != nil {
			return nil, fmt.Errorf("%w for collection %s: %v", ErrDriverInitFailed, collectionName, err)
		}

		collections[collectionName] = sc
	}

	// Resolve all switch configurations
	for switchName, switchCfg := range cfg.Switches {
		if switchName == "" {
			return nil, fmt.Errorf("switch name cannot be empty")
		}

		resolved, err := resolveSwitch(switchName, switchCfg, collections)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve switch %s: %w", switchName, err)
		}

		switches[switchName] = resolved
	}

	// Create switch groups
	for groupName, groupCfg := range cfg.Groups {
		if groupName == "" {
			return nil, fmt.Errorf("group name cannot be empty")
		}

		groupSwitches := make(map[string]*ResolvedSwitch)
		for _, switchName := range groupCfg.Switches {
			resolvedSwitch, exists := switches[switchName]
			if !exists {
				return nil, fmt.Errorf("switch %s not found for group %s", switchName, groupName)
			}
			groupSwitches[switchName] = resolvedSwitch
		}

		groups[groupName] = NewSwitchGroup(groupName, groupSwitches)
	}

	listenAddr := fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort)
	return newServerWithCollections(collections, switches, groups, listenAddr, true), nil
}

// resolveSwitch parses a switch spec and resolves it to a specific switch in a collection.
func resolveSwitch(switchName string, switchCfg SwitchConfig, collections map[string]switchcollection.SwitchCollection) (*ResolvedSwitch, error) {
	// Parse spec format: "collection_name.index"
	parts := strings.Split(switchCfg.Spec, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid switch spec format: %s (expected format: collection.index)", switchCfg.Spec)
	}

	collectionName := parts[0]
	indexStr := parts[1]

	collection, exists := collections[collectionName]
	if !exists {
		return nil, fmt.Errorf("collection %s not found for switch %s", collectionName, switchName)
	}

	index, err := strconv.ParseUint(indexStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid switch index %s for switch %s: %w", indexStr, switchName, err)
	}

	switchIndex := uint(index)
	if switchIndex >= collection.CountSwitches() {
		return nil, fmt.Errorf("switch index %d out of range for collection %s (max: %d) for switch %s",
			switchIndex, collectionName, collection.CountSwitches()-1, switchName)
	}

	sw, err := collection.GetSwitch(switchIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to get switch %d from collection %s for switch %s: %w",
			switchIndex, collectionName, switchName, err)
	}

	return &ResolvedSwitch{
		Name:       switchName,
		Collection: collection,
		Index:      switchIndex,
		Switch:     sw,
	}, nil
}

// newServerWithCollections creates a new Server instance with the given collections and switches.
// If addProductionMiddleware is true, adds logger and CORS middleware.
func newServerWithCollections(collections map[string]switchcollection.SwitchCollection, switches map[string]*ResolvedSwitch, groups map[string]*SwitchGroup, listenAddr string, addProductionMiddleware bool) *Server {
	s := &Server{
		listenAddr:  listenAddr,
		collections: collections,
		switches:    switches,
		groups:      groups,
		timers:      make(map[string]*timerData),
		blinkers:    make(map[string]*blink.Blink),
		flipflops:   make(map[string]*flipflop.Flipflop),
		router:      chi.NewRouter(),
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
		// GET endpoints for status queries - only need basic name validation for status
		r.With(
			s.validateJSONRequest,
			s.validateSwitchName,
			s.validateSwitchExists,
		).Get("/{name}", s.switchStatusHandler)

		// POST endpoints for switch control - restore full validation middleware chain
		r.With(
			s.validateJSONRequest,
			s.validateSwitchName,
			s.validateSwitchExists,
			s.validateSwitchRequest,
		).Post("/{name}", s.switchHandler)
	})
}

// Start starts the API server.

func (s *Server) Start() error {
	// Initialize all switch collections to off
	for name, collection := range s.collections {
		if err := collection.TurnOff(); err != nil {
			return fmt.Errorf("%w for collection %s: %v", ErrSwitchInitFailed, name, err)
		}
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

// Close closes all switch collection connections.

func (s *Server) Close() error {
	var errors []error
	for name, collection := range s.collections {
		if err := collection.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close collection %s: %w", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing collections: %v", errors)
	}
	return nil
}

func (s *Server) ListRoutes() [][]string {
	routes := [][]string{}

	chi.Walk(s.router, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error { //nolint:errcheck
		routes = append(routes, []string{method, route})
		return nil
	})

	return routes
}
