package soundboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server represents the HTTP server for the soundboard
type Server struct {
	config          *Config
	soundManager    *SoundManager
	audioPlayer     *AudioPlayer
	router          *chi.Mux
	server          *http.Server
	scanTicker      *time.Ticker
	scanCancel      context.CancelFunc
	scanWg          sync.WaitGroup
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
	json.NewEncoder(w).Encode(response)
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
	json.NewEncoder(w).Encode(response)
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
	json.NewEncoder(w).Encode(response)
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
		json.NewEncoder(w).Encode(response)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"volume":  req.Volume,
		"message": "Volume set successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
	json.NewEncoder(w).Encode(response)
}

// handleSoundsStatus returns information about the sound directory status
func (s *Server) handleSoundsStatus(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"soundCount":    s.soundManager.GetSoundCount(),
		"lastScanTime":  s.soundManager.GetLastScanTime().Unix(),
		"scanInterval":  s.config.ScanInterval,
		"soundDirectory": s.config.SoundDirectory,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleIndex serves the main soundboard page
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(s.getIndexHTML()))
}

// getIndexHTML returns the HTML for the soundboard interface
func (s *Server) getIndexHTML() string {
	baseURL := s.config.GetBaseURL()
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Soundboard</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 20px;
            background-color: #f5f5f5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background-color: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        
        /* Mobile container adjustments */
        @media (max-width: 767px) {
            body {
                margin: 10px;
            }
            
            .container {
                padding: 15px;
                border-radius: 5px;
            }
            
            h1 {
                font-size: 24px;
                margin-bottom: 20px;
            }
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
            padding: 20px;
            background-color: #f8f9fa;
            border-radius: 8px;
            border: 1px solid #e9ecef;
        }
        
        h1 {
            color: #333;
            margin: 0 0 10px 0;
            font-size: 2.2em;
        }
        
        .connection-status {
            font-size: 1em;
            font-weight: 500;
            color: #6c757d;
        }
        
        .connection-status.connected {
            color: #28a745;
        }
        
        .connection-status.error {
            color: #dc3545;
        }
        
        #message-container {
            min-height: 50px;
            margin-bottom: 20px;
            display: flex;
            align-items: center;
        }
        
        .message {
            width: 100%;
            padding: 15px;
            border-radius: 8px;
            font-weight: 500;
        }
        
        .message.error {
            background: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
        }
        
        .message.success {
            background: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
        }
        
        .message.warning {
            background: #fff3cd;
            color: #856404;
            border: 1px solid #ffeaa7;
        }
        .controls {
            display: grid;
            grid-template-columns: 1fr;
            gap: 15px;
            margin-bottom: 20px;
            padding: 15px;
            background-color: #f8f9fa;
            border-radius: 5px;
        }
        
        /* Responsive grid for larger screens */
        @media (min-width: 480px) {
            .controls {
                grid-template-columns: 1fr 1fr;
                gap: 20px;
            }
        }
        
        @media (min-width: 768px) {
            .controls {
                grid-template-columns: auto auto auto 1fr;
                align-items: center;
                gap: 20px;
            }
        }
        
        @media (min-width: 1024px) {
            .controls {
                display: flex;
                justify-content: space-between;
                align-items: center;
            }
        }
        .pagination {
            display: flex;
            gap: 10px;
            align-items: center;
            justify-self: end;
        }
        .pagination button {
            padding: 8px 16px;
            background-color: #007bff;
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
            min-height: 36px;
        }
        
        /* Mobile pagination */
        @media (max-width: 767px) {
            .pagination {
                justify-self: stretch;
                justify-content: center;
                flex-wrap: wrap;
                gap: 8px;
                width: 100%;
            }
            
            .pagination button {
                padding: 12px 20px;
                font-size: 16px;
                min-height: 44px; /* Touch-friendly size */
                flex: 1;
                min-width: 80px;
            }
            
            .pagination span {
                order: -1;
                width: 100%;
                text-align: center;
                font-weight: bold;
                margin-bottom: 8px;
            }
        }
        .pagination button:disabled {
            background-color: #6c757d;
            cursor: not-allowed;
        }
        .pagination button:hover:not(:disabled) {
            background-color: #0056b3;
        }
        .per-page-selector, .playback-mode-selector, .volume-control {
            display: flex;
            align-items: center;
            gap: 10px;
            flex-wrap: wrap;
        }
        
        .per-page-selector select, .playback-mode-selector select {
            padding: 8px 12px;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 14px;
            min-width: 80px;
        }
        
        /* Mobile-friendly control styling */
        @media (max-width: 767px) {
            .per-page-selector, .playback-mode-selector, .volume-control {
                flex-direction: column;
                align-items: flex-start;
                gap: 5px;
                width: 100%;
            }
            
            .per-page-selector select, .playback-mode-selector select {
                width: 100%;
                padding: 12px;
                font-size: 16px; /* Prevents zoom on iOS */
            }
            
            .per-page-selector label, .playback-mode-selector label, .volume-control label {
                font-weight: bold;
                font-size: 14px;
            }
        }
        .volume-control input[type="range"] {
            width: 120px;
            height: 6px;
            border-radius: 5px;
            background: #ddd;
            outline: none;
            opacity: 0.7;
            transition: opacity 0.2s;
        }
        .volume-control input[type="range"]:hover {
            opacity: 1;
        }
        
        /* Mobile volume control */
        @media (max-width: 767px) {
            .volume-control input[type="range"] {
                width: 100%;
                height: 8px;
                margin: 5px 0;
            }
            
            .volume-control {
                width: 100%;
            }
            
            .volume-value {
                align-self: flex-end;
            }
        }
        .volume-control input[type="range"]::-webkit-slider-thumb {
            appearance: none;
            width: 18px;
            height: 18px;
            border-radius: 50%;
            background: #007bff;
            cursor: pointer;
        }
        .volume-control input[type="range"]::-moz-range-thumb {
            width: 18px;
            height: 18px;
            border-radius: 50%;
            background: #007bff;
            cursor: pointer;
            border: none;
        }
        
        /* Mobile slider thumb */
        @media (max-width: 767px) {
            .volume-control input[type="range"]::-webkit-slider-thumb {
                width: 24px;
                height: 24px;
            }
            .volume-control input[type="range"]::-moz-range-thumb {
                width: 24px;
                height: 24px;
            }
        }
        .volume-value {
            font-size: 12px;
            color: #666;
            min-width: 30px;
        }
        .server-status {
            font-size: 12px;
            padding: 2px 6px;
            border-radius: 3px;
            font-weight: bold;
            white-space: nowrap;
        }
        
        /* Mobile server status */
        @media (max-width: 767px) {
            .server-status {
                font-size: 14px;
                padding: 4px 8px;
                margin-top: 5px;
            }
        }
        .server-status.available {
            background-color: #d4edda;
            color: #155724;
        }
        .server-status.unavailable {
            background-color: #f8d7da;
            color: #721c24;
        }
        .soundboard {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
            gap: 15px;
            margin-bottom: 20px;
        }
        .sound-button {
            padding: 15px;
            background-color: #28a745;
            color: white;
            border: none;
            border-radius: 8px;
            cursor: pointer;
            font-size: 14px;
            font-weight: bold;
            text-align: center;
            word-wrap: break-word;
            transition: all 0.2s ease;
            position: relative;
            overflow: hidden;
        }
        .sound-button:hover {
            background-color: #218838;
            transform: translateY(-1px);
            box-shadow: 0 4px 8px rgba(0,0,0,0.2);
        }
        .sound-button:active {
            background-color: #1e7e34;
            transform: translateY(1px);
        }
        .sound-button.playing {
            background-color: #dc3545;
            animation: pulse 1.5s ease-in-out infinite;
            box-shadow: 0 0 15px rgba(220, 53, 69, 0.5);
        }
        .sound-button.playing:hover {
            background-color: #c82333;
        }
        .sound-button.playing::before {
            content: "⏸ ";
            font-size: 12px;
        }
        @keyframes pulse {
            0% { box-shadow: 0 0 15px rgba(220, 53, 69, 0.5); }
            50% { box-shadow: 0 0 25px rgba(220, 53, 69, 0.8); }
            100% { box-shadow: 0 0 15px rgba(220, 53, 69, 0.5); }
        }
        .loading {
            text-align: center;
            padding: 40px;
            color: #666;
        }
        
        .container.loading {
            opacity: 0.6;
            pointer-events: none;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Soundboard</h1>
            <div class="connection-status" id="connection-status">Connecting...</div>
        </div>
        
        <div class="controls">
            <div class="per-page-selector">
                <label for="perPage">Items per page:</label>
                <select id="perPage">
                    <option value="10">10</option>
                    <option value="20" selected>20</option>
                    <option value="50">50</option>
                    <option value="100">100</option>
                </select>
            </div>

            <div class="playback-mode-selector">
                <label for="playbackMode">Audio playback:</label>
                <select id="playbackMode">
                    <option value="browser">Browser</option>
                    <option value="server">Server</option>
                </select>
                <span id="serverStatus" class="server-status"></span>
            </div>

            <div class="volume-control">
                <label for="volumeSlider">Volume:</label>
                <input type="range" id="volumeSlider" min="0" max="100" value="70" step="1">
                <span id="volumeValue" class="volume-value">70%</span>
            </div>
            
            <div class="pagination">
                <button id="prevPage" disabled>Previous</button>
                <span id="pageInfo">Page 1 of 1</span>
                <button id="nextPage" disabled>Next</button>
            </div>
        </div>

        <div id="message-container"></div>
        <div id="loadingMessage" class="loading">Loading sounds...</div>
        <div id="soundboard" class="soundboard"></div>
    </div>

    <script>
        // Base URL injected from server
        const BASE_URL = '%s';
        
        class Soundboard {
            constructor() {
                this.currentPage = 1;
                this.totalPages = 1;
                this.perPage = 20;
                this.sounds = [];
                this.currentAudio = null;
                this.currentPlayingButton = null;
                this.serverAvailable = false;
                this.baseURL = BASE_URL;
                this.statusCheckInterval = null;
                this.volume = 70; // Default volume 70%
                this.userAdjustingVolume = false; // Track if user is actively adjusting volume
                this.lastSoundCount = 0; // Track sound count for change detection
                this.soundUpdateInterval = null; // Track update checking interval
                this.isLoading = false; // Track loading state
                this.connectionStatus = 'connecting'; // Track connection status
                
                // Initialize playback mode from localStorage or dropdown
                this.initializePlaybackMode();
                
                this.setupEventListeners();
                this.checkServerStatus();
                this.loadSounds();
                this.startSoundUpdateChecking();
            }

            // Initialize playback mode from localStorage or dropdown value
            initializePlaybackMode() {
                const playbackModeSelect = document.getElementById('playbackMode');
                
                // Try to restore from localStorage first
                const savedMode = localStorage.getItem('soundboard-playback-mode');
                if (savedMode && (savedMode === 'browser' || savedMode === 'server')) {
                    this.playbackMode = savedMode;
                    playbackModeSelect.value = savedMode;
                } else {
                    // Use current dropdown value as fallback
                    this.playbackMode = playbackModeSelect.value || 'browser';
                }
                
                // Initialize mode-specific features after setting playback mode
                this.initializeModeFeatures();
            }
            
            // Initialize features specific to the current playback mode
            initializeModeFeatures() {
                if (this.playbackMode === 'server') {
                    // Defer server mode initialization until after server status is checked
                    setTimeout(() => {
                        if (this.serverAvailable) {
                            this.startStatusPolling();
                            this.syncServerVolume();
                        }
                    }, 100);
                }
            }

            // Helper method to build URLs with base URL
            buildURL(path) {
                if (this.baseURL) {
                    return this.baseURL + path;
                }
                return path;
            }

            setupEventListeners() {
                document.getElementById('prevPage').addEventListener('click', () => {
                    if (this.currentPage > 1) {
                        this.currentPage--;
                        this.loadSounds();
                    }
                });

                document.getElementById('nextPage').addEventListener('click', () => {
                    if (this.currentPage < this.totalPages) {
                        this.currentPage++;
                        this.loadSounds();
                    }
                });

                document.getElementById('perPage').addEventListener('change', (e) => {
                    this.perPage = parseInt(e.target.value);
                    this.currentPage = 1;
                    this.loadSounds();
                });

                document.getElementById('playbackMode').addEventListener('change', (e) => {
                    this.playbackMode = e.target.value;
                    
                    // Save the selection to localStorage
                    localStorage.setItem('soundboard-playback-mode', this.playbackMode);
                    
                    // Stop any currently playing sound when changing modes
                    this.stopCurrentSound();
                    
                    // Start/stop status polling based on mode
                    if (this.playbackMode === 'server') {
                        this.startStatusPolling();
                        this.syncServerVolume();
                    } else {
                        this.stopStatusPolling();
                    }
                });

                const volumeSlider = document.getElementById('volumeSlider');
                
                // Handle volume slider input with debouncing
                volumeSlider.addEventListener('input', (e) => {
                    this.userAdjustingVolume = true;
                    this.volume = parseInt(e.target.value);
                    this.updateVolumeDisplay();
                    this.applyVolumeChange();
                    
                    // Clear any existing timeout
                    if (this.volumeDebounceTimeout) {
                        clearTimeout(this.volumeDebounceTimeout);
                    }
                    
                    // Reset flag after user stops adjusting (500ms delay)
                    this.volumeDebounceTimeout = setTimeout(() => {
                        this.userAdjustingVolume = false;
                    }, 500);
                });
            }

            async checkServerStatus() {
                try {
                    const response = await fetch(this.buildURL('/api/audio/info'));
                    if (response.ok) {
                        const data = await response.json();
                        this.serverAvailable = data.serverAvailable;
                        this.updateServerStatus(data);
                        this.updateConnectionStatus('Connected');
                    } else {
                        throw new Error('HTTP ' + response.status + ': ' + response.statusText);
                    }
                } catch (error) {
                    console.warn('Failed to check server audio status:', error);
                    this.serverAvailable = false;
                    this.updateServerStatus(null);
                    this.updateConnectionStatus('Connection Error');
                    this.showMessage('Unable to connect to server: ' + error.message, 'error');
                }
            }

            updateServerStatus(audioInfo) {
                const statusElement = document.getElementById('serverStatus');
                const playbackModeSelect = document.getElementById('playbackMode');
                
                if (this.serverAvailable) {
                    statusElement.textContent = '✓ Available';
                    statusElement.className = 'server-status available';
                    playbackModeSelect.disabled = false;
                } else {
                    statusElement.textContent = '✗ Unavailable';
                    statusElement.className = 'server-status unavailable';
                    // Disable server option and force browser mode
                    playbackModeSelect.disabled = false; // Keep enabled for user awareness
                    if (this.playbackMode === 'server') {
                        this.playbackMode = 'browser';
                        playbackModeSelect.value = 'browser';
                    }
                }

                // Show additional info if available
                if (audioInfo && audioInfo.availablePlayers) {
                    const players = Object.entries(audioInfo.availablePlayers)
                        .filter(([, available]) => available)
                        .map(([name]) => name);
                    
                    if (players.length > 0) {
                        statusElement.title = ` + "`Available players: ${players.join(', ')}`" + `;
                    }
                }
            }

            startStatusPolling() {
                // Clear any existing interval
                this.stopStatusPolling();
                
                // Poll server status and volume every 1 second when in server mode
                this.statusCheckInterval = setInterval(() => {
                    this.checkServerPlaybackStatus();
                    this.syncServerVolume();
                }, 1000);
            }

            stopStatusPolling() {
                if (this.statusCheckInterval) {
                    clearInterval(this.statusCheckInterval);
                    this.statusCheckInterval = null;
                }
            }

            async syncServerVolume() {
                // Get current server volume and sync slider from audio info
                if (this.playbackMode === 'server' && !this.userAdjustingVolume) {
                    try {
                        const response = await fetch(this.buildURL('/api/audio/info'));
                        if (response.ok) {
                            const data = await response.json();
                            this.updateConnectionStatus('Connected');
                            if (data.volumeSuccess && data.volume !== undefined) {
                                // Only update if volume has actually changed
                                if (this.volume !== data.volume) {
                                    this.volume = data.volume;
                                    document.getElementById('volumeSlider').value = this.volume;
                                    this.updateVolumeDisplay();
                                }
                            }
                        } else {
                            this.updateConnectionStatus('Connection Error');
                        }
                    } catch (error) {
                        console.warn('Failed to sync server volume:', error);
                        this.updateConnectionStatus('Connection Error');
                    }
                }
            }

            updateVolumeDisplay() {
                document.getElementById('volumeValue').textContent = this.volume + '%';
            }

            async applyVolumeChange() {
                if (this.playbackMode === 'browser') {
                    // Apply volume to current browser audio
                    if (this.currentAudio) {
                        this.currentAudio.volume = this.volume / 100;
                    }
                } else {
                    // Send volume change to server for ALSA
                    try {
                        const response = await fetch(this.buildURL('/api/audio/volume'), {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json',
                            },
                            body: JSON.stringify({ volume: this.volume })
                        });
                        if (response.ok) {
                            this.updateConnectionStatus('Connected');
                        } else {
                            throw new Error('HTTP ' + response.status + ': ' + response.statusText);
                        }
                    } catch (error) {
                        console.warn('Failed to set server volume:', error);
                        this.updateConnectionStatus('Connection Error');
                    }
                }
            }

            async checkServerPlaybackStatus() {
                if (this.playbackMode !== 'server') {
                    return;
                }
                
                try {
                    const response = await fetch(this.buildURL('/api/audio/info'));
                    if (response.ok) {
                        const data = await response.json();
                        this.updateConnectionStatus('Connected');
                        
                        // Check if server playback has stopped
                        if (!data.isPlaying && this.currentPlayingButton) {
                            this.currentPlayingButton.classList.remove('playing');
                            this.currentPlayingButton = null;
                        }
                        
                        // Show any errors
                        if (data.lastError) {
                            console.warn('Server audio error:', data.lastError);
                            // Remove playing state if there's an error
                            if (this.currentPlayingButton) {
                                this.currentPlayingButton.classList.remove('playing');
                                this.currentPlayingButton = null;
                            }
                        }
                    } else {
                        this.updateConnectionStatus('Connection Error');
                    }
                } catch (error) {
                    console.warn('Failed to check server playback status:', error);
                    this.updateConnectionStatus('Connection Error');
                }
            }

            async loadSounds() {
                try {
                    // Stop any currently playing sound when loading new page
                    this.stopCurrentSound();
                    
                    this.setLoading(true);
                    this.hideMessages();
                    
                    const response = await fetch(this.buildURL(` + "`/api/sounds?page=${this.currentPage}&per_page=${this.perPage}`" + `));
                    
                    if (!response.ok) {
                        throw new Error(` + "`HTTP ${response.status}: ${response.statusText}`" + `);
                    }
                    
                    const data = await response.json();
                    this.sounds = data.sounds;
                    this.currentPage = data.currentPage;
                    this.totalPages = data.totalPages;
                    this.perPage = data.itemsPerPage;
                    
                    this.renderSounds();
                    this.updatePagination();
                    this.updateConnectionStatus('Connected');
                    
                } catch (error) {
                    console.error('Error loading sounds:', error);
                    this.showMessage(` + "`Failed to load sounds: ${error.message}`" + `, 'error');
                    this.updateConnectionStatus('Connection Error');
                    // Show empty state
                    this.renderSounds();
                } finally {
                    this.setLoading(false);
                    this.hideLoading();
                }
            }

            showLoading() {
                document.getElementById('loadingMessage').style.display = 'block';
                document.getElementById('soundboard').style.display = 'none';
                document.getElementById('errorMessage').style.display = 'none';
                document.getElementById('infoMessage').style.display = 'none';
            }

            hideLoading() {
                document.getElementById('loadingMessage').style.display = 'none';
                document.getElementById('soundboard').style.display = 'grid';
            }

            // Legacy methods - now use showMessage instead
            showError(message) {
                this.showMessage(message, 'error');
            }

            showInfo(message) {
                this.showMessage(message, 'warning');
            }

            renderSounds() {
                const soundboard = document.getElementById('soundboard');
                soundboard.innerHTML = '';

                if (this.sounds.length === 0) {
                    this.showInfo('No sounds found. Add some audio files to the sounds directory.');
                    return;
                }

                this.sounds.forEach(sound => {
                    const button = document.createElement('button');
                    button.className = 'sound-button';
                    button.textContent = sound.displayName;
                    button.title = ` + "`Play/Stop ${sound.displayName}`" + `;
                    
                    button.addEventListener('click', () => {
                        this.playSound(sound, button);
                    });
                    
                    soundboard.appendChild(button);
                });
            }

            stopCurrentSound() {
                // Stop browser audio if playing
                if (this.currentAudio) {
                    this.currentAudio.pause();
                    this.currentAudio.currentTime = 0;
                    this.currentAudio = null;
                }
                
                // Stop server audio (always safe to call)
                fetch(this.buildURL('/api/sounds/stop'), {
                    method: 'POST'
                }).catch(err => console.warn('Failed to stop server audio:', err));
                
                // Remove visual feedback
                if (this.currentPlayingButton) {
                    this.currentPlayingButton.classList.remove('playing');
                    this.currentPlayingButton = null;
                }
            }

            async playSound(sound, button) {
                try {
                    // If this button is already playing, stop it
                    if (this.currentPlayingButton === button) {
                        this.stopCurrentSound();
                        return;
                    }
                    
                    // Stop any currently playing sound
                    this.stopCurrentSound();
                    
                    // Add visual feedback immediately
                    button.classList.add('playing');
                    this.currentPlayingButton = button;
                    
                    if (this.playbackMode === 'server') {
                        // Server-side playback
                        const response = await fetch(this.buildURL(` + "`/api/sounds/${sound.fileName}/play?mode=server`" + `), {
                            method: 'POST'
                        });
                        
                        if (!response.ok) {
                            throw new Error(` + "`Server playback failed: ${response.statusText}`" + `);
                        }
                        
                        const result = await response.json();
                        console.log('Server playback result:', result);
                        this.updateConnectionStatus('Connected');
                        
                        // Check if server playback actually started
                        if (result.error || !result.serverPlayback) {
                            throw new Error(result.error || 'Server playback failed to start');
                        }
                        
                        // Start status polling to detect when playback ends
                        if (!this.statusCheckInterval) {
                            this.startStatusPolling();
                        }
                        
                    } else {
                        // Browser-side playback
                        const audio = new Audio(this.buildURL(` + "`/sounds/${sound.fileName}`" + `));
                        audio.volume = this.volume / 100; // Apply current volume
                        this.currentAudio = audio;
                        
                        // Set up event listeners
                        audio.addEventListener('ended', () => {
                            this.stopCurrentSound();
                        });
                        
                        audio.addEventListener('error', (e) => {
                            console.error('Audio error:', e);
                            this.stopCurrentSound();
                            alert('Failed to play sound: ' + sound.displayName);
                        });
                        
                        // Play the audio
                        await audio.play();
                        this.updateConnectionStatus('Connected');
                        
                        // Send play event to server for logging
                        fetch(this.buildURL(` + "`/api/sounds/${sound.fileName}/play?mode=browser`" + `), {
                            method: 'POST'
                        }).catch(err => console.warn('Failed to log play event:', err));
                    }
                    
                } catch (error) {
                    console.error('Error playing sound:', error);
                    this.stopCurrentSound();
                    this.showMessage(` + "`Failed to play sound '${sound.displayName}': ${error.message}`" + `, 'error');
                    if (error.message.includes('HTTP') || error.message.includes('fetch')) {
                        this.updateConnectionStatus('Connection Error');
                    }
                }
            }

            updatePagination() {
                const prevButton = document.getElementById('prevPage');
                const nextButton = document.getElementById('nextPage');
                const pageInfo = document.getElementById('pageInfo');
                const perPageSelect = document.getElementById('perPage');

                prevButton.disabled = this.currentPage <= 1;
                nextButton.disabled = this.currentPage >= this.totalPages;
                pageInfo.textContent = ` + "`Page ${this.currentPage} of ${this.totalPages}`" + `;
                perPageSelect.value = this.perPage.toString();
            }

            // Start checking for sound directory updates
            startSoundUpdateChecking() {
                // Check every 5 seconds for updates (more frequent than server scanning)
                this.soundUpdateInterval = setInterval(() => {
                    this.checkForSoundUpdates();
                }, 5000);
            }

            // Stop checking for sound directory updates
            stopSoundUpdateChecking() {
                if (this.soundUpdateInterval) {
                    clearInterval(this.soundUpdateInterval);
                    this.soundUpdateInterval = null;
                }
            }

            // Check if the sound directory has been updated
            async checkForSoundUpdates() {
                try {
                    const response = await fetch(this.buildURL('/api/sounds/status'));
                    if (response.ok) {
                        const data = await response.json();
                        this.updateConnectionStatus('Connected');
                        
                        // Check if sound count has changed
                        if (this.lastSoundCount > 0 && this.lastSoundCount !== data.soundCount) {
                            console.log('Sound directory updated: ' + this.lastSoundCount + ' -> ' + data.soundCount + ' sounds');
                            this.showMessage('Sound directory updated: ' + data.soundCount + ' sounds available', 'success');
                            
                            // Reload the current page of sounds
                            this.loadSounds();
                        }
                        
                        this.lastSoundCount = data.soundCount;
                    } else {
                        this.updateConnectionStatus('Connection Error');
                    }
                } catch (error) {
                    console.warn('Failed to check for sound updates:', error);
                    this.updateConnectionStatus('Connection Error');
                }
            }

            // Set loading state
            setLoading(loading) {
                this.isLoading = loading;
                const container = document.querySelector('.container');
                if (loading) {
                    container.classList.add('loading');
                } else {
                    container.classList.remove('loading');
                }
            }

            // Show temporary message
            showMessage(message, type) {
                const messageContainer = document.getElementById('message-container');
                messageContainer.innerHTML = ` + "`<div class=\"message ${type}\">${message}</div>`" + `;
                setTimeout(() => {
                    messageContainer.innerHTML = '';
                }, type === 'error' ? 8000 : 4000);
            }

            // Hide messages
            hideMessages() {
                const messageContainer = document.getElementById('message-container');
                messageContainer.innerHTML = '';
            }

            // Update connection status indicator
            updateConnectionStatus(status) {
                this.connectionStatus = status.toLowerCase();
                const statusElement = document.getElementById('connection-status');
                statusElement.textContent = status;
                
                // Remove existing status classes
                statusElement.classList.remove('connected', 'error');
                
                // Add appropriate status class
                if (status === 'Connected') {
                    statusElement.classList.add('connected');
                } else if (status.includes('Error') || status.includes('Failed')) {
                    statusElement.classList.add('error');
                }
            }
        }

        // Initialize the soundboard when the page loads
        document.addEventListener('DOMContentLoaded', () => {
            new Soundboard();
        });
    </script>
</body>
</html>`
	
	// Replace the BASE_URL placeholder with the actual base URL
	return strings.ReplaceAll(html, "const BASE_URL = '%s';", fmt.Sprintf("const BASE_URL = '%s';", baseURL))
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
