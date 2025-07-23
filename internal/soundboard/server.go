package soundboard

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/larsks/airdancer/internal/static"
)

// Server represents the HTTP server for the soundboard
type Server struct {
	config       *Config
	soundManager *SoundManager
	audioPlayer  *AudioPlayer
	router       *chi.Mux
	server       *http.Server
	scanTicker   *time.Ticker
	scanCancel   context.CancelFunc
	scanWg       sync.WaitGroup
}

// SoundsResponse represents the API response for sounds listing
type SoundsResponse struct {
	Sounds       []Sound `json:"sounds"`
	CurrentPage  int     `json:"currentPage"`
	TotalPages   int     `json:"totalPages"`
	ItemsPerPage int     `json:"itemsPerPage"`
}

// NewServer creates a new soundboard server
func NewServer(config *Config) (*Server, error) {
	soundManager := NewSoundManager(config.SoundDirectory)

	// Load sounds at startup
	if err := soundManager.LoadSounds(); err != nil {
		return nil, fmt.Errorf("failed to load sounds: %w", err)
	}

	audioPlayer := NewAudioPlayer(config)

	s := &Server{
		config:       config,
		soundManager: soundManager,
		audioPlayer:  audioPlayer,
	}

	s.setupRoutes()
	s.startBackgroundScanning()
	return s, nil
}

// setupRoutes configures the HTTP routes
func (s *Server) setupRoutes() {
	s.router = chi.NewRouter()

	// Add CORS middleware
	s.router.Use(s.corsMiddleware)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)

	baseURL := s.config.GetBaseURL()

	if baseURL != "" {
		// Mount everything under the base URL path
		s.router.Route(baseURL, func(r chi.Router) {
			s.setupSubRoutes(r)
		})
	} else {
		// Mount directly on root
		s.setupSubRoutes(s.router)
	}
}

// setupSubRoutes configures the actual routes (used both for root and base URL mounting)
func (s *Server) setupSubRoutes(r chi.Router) {
	// API routes
	r.Route("/api", func(apiRouter chi.Router) {
		apiRouter.Get("/sounds", s.handleSounds)
		apiRouter.Post("/sounds/{filename}/play", s.handlePlaySound)
		apiRouter.Post("/sounds/stop", s.handleStopSound)
		apiRouter.Get("/audio/info", s.handleAudioInfo)
		apiRouter.Post("/audio/volume", s.handleSetVolume)
		apiRouter.Post("/sounds/rescan", s.handleRescanSounds)
		apiRouter.Get("/sounds/status", s.handleSoundsStatus)
	})

	// Static file serving for sound files
	r.Handle("/sounds/*", http.StripPrefix(s.config.GetFullPath("/sounds/"), http.FileServer(http.Dir(s.config.SoundDirectory))))

	// Frontend route (serves the main page)
	r.Get("/", s.handleIndex)
}

// corsMiddleware adds CORS headers to allow browser audio playback
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleSounds returns a paginated list of sounds
func (s *Server) handleSounds(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Parse page parameter
	page := 1
	if pageStr := query.Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Parse per_page parameter
	perPage := s.config.ItemsPerPage
	if perPageStr := query.Get("per_page"); perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
			perPage = pp
		}
	}

	// Get paginated sounds
	sounds, totalPages, err := s.soundManager.GetSoundsPage(page, perPage)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting sounds: %v", err), http.StatusBadRequest)
		return
	}

	response := SoundsResponse{
		Sounds:       sounds,
		CurrentPage:  page,
		TotalPages:   totalPages,
		ItemsPerPage: perPage,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// handlePlaySound handles sound playback requests
func (s *Server) handlePlaySound(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")

	// Parse playback mode from query parameter (default to browser)
	playbackMode := r.URL.Query().Get("mode")
	if playbackMode == "" {
		playbackMode = "browser"
	}

	// Find the sound by filename
	var targetSound *Sound
	for _, sound := range s.soundManager.GetSounds() {
		if sound.FileName == filename {
			targetSound = &sound
			break
		}
	}

	if targetSound == nil {
		http.Error(w, "Sound not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"message":        "Sound play request received",
		"filename":       filename,
		"displayName":    targetSound.DisplayName,
		"playbackMode":   playbackMode,
		"url":            s.config.GetFullPath(fmt.Sprintf("/sounds/%s", filename)),
		"serverPlayback": false,
	}

	// If server-side playback is requested, play the sound on the server
	if playbackMode == "server" {
		// Clear any previous errors
		s.audioPlayer.ClearLastError()

		if err := s.audioPlayer.PlaySound(targetSound.FilePath); err != nil {
			response["serverPlayback"] = false
			response["error"] = err.Error()
			response["message"] = "Failed to start sound playback on server"
			// Don't return HTTP error, let frontend handle it
		} else {
			response["serverPlayback"] = true
			response["message"] = "Sound playing on server"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response) //nolint:errcheck
}

// handleStopSound handles stopping currently playing sounds
func (s *Server) handleStopSound(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"message":        "Stop request received",
		"serverPlayback": false,
	}

	// Always try to stop server-side playback (no harm if nothing is playing)
	if err := s.audioPlayer.StopCurrentSound(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to stop sound on server: %v", err), http.StatusInternalServerError)
		return
	}
	response["serverPlayback"] = true
	response["message"] = "Sound stopped on server"

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response) //nolint:errcheck
}

// handleAudioInfo provides comprehensive information about audio configuration, status, and volume
func (s *Server) handleAudioInfo(w http.ResponseWriter, r *http.Request) {
	playbackStatus := s.audioPlayer.GetPlaybackStatus()

	response := map[string]interface{}{
		"alsaDevice":       s.config.ALSADevice,
		"alsaCardName":     s.config.ALSACardName,
		"serverAvailable":  s.audioPlayer.IsServerMode(),
		"availablePlayers": s.audioPlayer.GetAudioPlayerInfo(),
	}

	// Add volume information
	if volume, err := s.audioPlayer.GetVolume(); err == nil {
		response["volume"] = volume
		response["volumeSuccess"] = true
	} else {
		response["volume"] = 0
		response["volumeSuccess"] = false
		response["volumeError"] = err.Error()
	}

	// Merge playback status
	for key, value := range playbackStatus {
		response[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response) //nolint:errcheck
}

// handleSetVolume sets the ALSA volume
func (s *Server) handleSetVolume(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Volume int `json:"volume"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Volume < 0 || req.Volume > 100 {
		http.Error(w, "Volume must be between 0 and 100", http.StatusBadRequest)
		return
	}

	if err := s.audioPlayer.SetVolume(req.Volume); err != nil {
		response := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"volume":  req.Volume,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response) //nolint:errcheck
		return
	}

	response := map[string]interface{}{
		"success": true,
		"volume":  req.Volume,
		"message": "Volume set successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response) //nolint:errcheck
}

// handleRescanSounds manually triggers a rescan of the sound directory
func (s *Server) handleRescanSounds(w http.ResponseWriter, r *http.Request) {
	changed, err := s.soundManager.RescanDirectory()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to rescan directory: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"changed": changed,
		"message": "Directory rescanned successfully",
		"count":   s.soundManager.GetSoundCount(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response) //nolint:errcheck
}

// handleSoundsStatus returns information about the sound directory status
func (s *Server) handleSoundsStatus(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"soundCount":     s.soundManager.GetSoundCount(),
		"lastScanTime":   s.soundManager.GetLastScanTime().Unix(),
		"scanInterval":   s.config.ScanInterval,
		"soundDirectory": s.config.SoundDirectory,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response) //nolint:errcheck
}

// handleIndex serves the main soundboard page
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Get soundboard controller JavaScript and CSS
	soundboardControllerJS, err := static.GetSoundboardControllerJS()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get soundboard controller JS: %v", err), http.StatusInternalServerError)
		return
	}

	soundboardCSS, err := static.GetSoundboardCSS()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get soundboard CSS: %v", err), http.StatusInternalServerError)
		return
	}

	// Get soundboard content HTML
	contentHTML, err := static.GetSoundboardContent()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get soundboard content: %v", err), http.StatusInternalServerError)
		return
	}

	baseURL := s.config.GetBaseURL()

	// Prepare template data
	data := static.TemplateData{
		Title:         "Soundboard",
		DefaultStatus: "Connecting...",
		ExtraCSS:      template.HTML(fmt.Sprintf("<style>%s</style>", soundboardCSS)),
		Content:       template.HTML(contentHTML),
		ExtraJS: template.HTML(fmt.Sprintf(`
			<script>
				%s
				
				// Initialize the soundboard controller when the page loads
				document.addEventListener('DOMContentLoaded', () => {
					new SoundboardController('%s');
				});
			</script>
		`, soundboardControllerJS, baseURL)),
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

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.ListenAddress, s.config.ListenPort)
	s.server = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	fmt.Printf("Starting soundboard server on %s\n", addr)
	return s.server.ListenAndServe()
}

// Close shuts down the HTTP server
func (s *Server) Close() error {
	s.stopBackgroundScanning()
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// startBackgroundScanning starts the background directory scanning if enabled
func (s *Server) startBackgroundScanning() {
	if s.config.ScanInterval <= 0 {
		fmt.Printf("Directory scanning disabled (scan-interval: %d)\n", s.config.ScanInterval)
		return
	}

	fmt.Printf("Starting directory scanning every %d seconds\n", s.config.ScanInterval)

	ctx, cancel := context.WithCancel(context.Background())
	s.scanCancel = cancel
	s.scanTicker = time.NewTicker(time.Duration(s.config.ScanInterval) * time.Second)

	s.scanWg.Add(1)
	go func() {
		defer s.scanWg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.scanTicker.C:
				s.performBackgroundScan()
			}
		}
	}()
}

// stopBackgroundScanning stops the background directory scanning
func (s *Server) stopBackgroundScanning() {
	if s.scanCancel != nil {
		s.scanCancel()
	}
	if s.scanTicker != nil {
		s.scanTicker.Stop()
	}
	s.scanWg.Wait()
}

// performBackgroundScan performs a single background scan
func (s *Server) performBackgroundScan() {
	changed, err := s.soundManager.RescanDirectory()
	if err != nil {
		fmt.Printf("Background scan error: %v\n", err)
		return
	}

	if changed {
		fmt.Printf("Sound directory changes detected - %d sounds now available\n", s.soundManager.GetSoundCount())
	}
}
