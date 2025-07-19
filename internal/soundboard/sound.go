package soundboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Sound represents a sound file with its metadata
type Sound struct {
	// FileName is the actual filename of the sound file
	FileName string `json:"fileName"`
	// DisplayName is the name to show in the UI
	DisplayName string `json:"displayName"`
	// FilePath is the full path to the sound file
	FilePath string `json:"-"`
}

// SoundMetadata represents the optional JSON metadata file for a sound
type SoundMetadata struct {
	DisplayName string `json:"displayName"`
}

// SoundManager handles discovery and management of sound files
type SoundManager struct {
	soundDirectory string
	sounds         []Sound
	lastScanTime   time.Time
	mutex          sync.RWMutex
}

// NewSoundManager creates a new SoundManager
func NewSoundManager(soundDirectory string) *SoundManager {
	return &SoundManager{
		soundDirectory: soundDirectory,
		sounds:         make([]Sound, 0),
	}
}

// LoadSounds discovers and loads all sound files from the configured directory
func (sm *SoundManager) LoadSounds() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	return sm.loadSounds()
}

// loadSounds is the internal implementation (assumes mutex is held)
func (sm *SoundManager) loadSounds() error {
	// Check if directory exists
	if _, err := os.Stat(sm.soundDirectory); os.IsNotExist(err) {
		return fmt.Errorf("sound directory does not exist: %s", sm.soundDirectory)
	}

	// Clear existing sounds
	sm.sounds = make([]Sound, 0)

	// Walk through the directory to find sound files
	err := filepath.Walk(sm.soundDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if this is a sound file (common audio extensions)
		ext := strings.ToLower(filepath.Ext(path))
		if !sm.isSoundFile(ext) {
			return nil
		}

		// Create sound object
		sound := Sound{
			FileName: info.Name(),
			FilePath: path,
		}

		// Try to load metadata
		if err := sm.loadSoundMetadata(&sound); err != nil {
			// If metadata loading fails, use filename without extension as display name
			sound.DisplayName = sm.getFileNameWithoutExt(sound.FileName)
		}

		sm.sounds = append(sm.sounds, sound)
		return nil
	})

	if err == nil {
		sm.lastScanTime = time.Now()
	}

	return err
}

// isSoundFile checks if the file extension indicates it's a sound file
func (sm *SoundManager) isSoundFile(ext string) bool {
	soundExtensions := []string{
		".mp3", ".wav", ".ogg", ".m4a", ".aac", ".flac",
		".wma", ".opus", ".mp4", ".webm", ".3gp",
	}

	for _, soundExt := range soundExtensions {
		if ext == soundExt {
			return true
		}
	}
	return false
}

// loadSoundMetadata attempts to load metadata from a JSON file with the same basename
func (sm *SoundManager) loadSoundMetadata(sound *Sound) error {
	// Get the base name without extension
	baseName := sm.getFileNameWithoutExt(sound.FileName)
	metadataPath := filepath.Join(sm.soundDirectory, baseName+".json")

	// Check if metadata file exists
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		// No metadata file, use filename as display name
		sound.DisplayName = baseName
		return nil
	}

	// Read and parse metadata file
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		sound.DisplayName = baseName
		return fmt.Errorf("failed to read metadata file %s: %w", metadataPath, err)
	}

	var metadata SoundMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		sound.DisplayName = baseName
		return fmt.Errorf("failed to parse metadata file %s: %w", metadataPath, err)
	}

	// Use display name from metadata, fallback to filename if empty
	if metadata.DisplayName != "" {
		sound.DisplayName = metadata.DisplayName
	} else {
		sound.DisplayName = baseName
	}

	return nil
}

// getFileNameWithoutExt returns the filename without its extension
func (sm *SoundManager) getFileNameWithoutExt(fileName string) string {
	return strings.TrimSuffix(fileName, filepath.Ext(fileName))
}

// GetSounds returns all loaded sounds
func (sm *SoundManager) GetSounds() []Sound {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	// Return a copy to avoid race conditions
	sounds := make([]Sound, len(sm.sounds))
	copy(sounds, sm.sounds)
	return sounds
}

// GetSoundsPage returns a page of sounds for pagination
func (sm *SoundManager) GetSoundsPage(page, pageSize int) ([]Sound, int, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	totalSounds := len(sm.sounds)
	totalPages := (totalSounds + pageSize - 1) / pageSize

	if page > totalPages && totalPages > 0 {
		return nil, totalPages, fmt.Errorf("page %d exceeds total pages %d", page, totalPages)
	}

	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize

	if startIdx >= totalSounds {
		return []Sound{}, totalPages, nil
	}

	if endIdx > totalSounds {
		endIdx = totalSounds
	}

	return sm.sounds[startIdx:endIdx], totalPages, nil
}

// RescanDirectory rescans the sound directory and returns true if changes were found
func (sm *SoundManager) RescanDirectory() (bool, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	// Store the current sounds for comparison
	oldSounds := make([]Sound, len(sm.sounds))
	copy(oldSounds, sm.sounds)
	
	// Rescan the directory
	if err := sm.loadSounds(); err != nil {
		return false, err
	}
	
	// Compare the old and new sound lists
	return !sm.soundListsEqual(oldSounds, sm.sounds), nil
}

// soundListsEqual compares two sound slices for equality
func (sm *SoundManager) soundListsEqual(a, b []Sound) bool {
	if len(a) != len(b) {
		return false
	}
	
	// Create maps for comparison
	aMap := make(map[string]Sound)
	bMap := make(map[string]Sound)
	
	for _, sound := range a {
		aMap[sound.FileName] = sound
	}
	
	for _, sound := range b {
		bMap[sound.FileName] = sound
	}
	
	// Compare the maps
	for filename, soundA := range aMap {
		soundB, exists := bMap[filename]
		if !exists || soundA.DisplayName != soundB.DisplayName || soundA.FilePath != soundB.FilePath {
			return false
		}
	}
	
	return true
}

// GetLastScanTime returns the time of the last successful scan
func (sm *SoundManager) GetLastScanTime() time.Time {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.lastScanTime
}

// GetSoundCount returns the current number of sounds
func (sm *SoundManager) GetSoundCount() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return len(sm.sounds)
}
