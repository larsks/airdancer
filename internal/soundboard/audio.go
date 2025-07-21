package soundboard

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AudioPlayer handles server-side audio playback
type AudioPlayer struct {
	config           *Config
	currentProcess   *exec.Cmd
	currentSoundFile string
	playbackStarted  time.Time
	lastError        error
	mutex            sync.Mutex
}

// NewAudioPlayer creates a new AudioPlayer instance
func NewAudioPlayer(config *Config) *AudioPlayer {
	return &AudioPlayer{
		config: config,
	}
}

// IsServerMode returns true if audio playback should happen on the server
// This is now determined by the API call, not configuration
func (ap *AudioPlayer) IsServerMode() bool {
	return true // Audio player is always available for server-side playback
}

// PlaySound plays a sound file using the configured ALSA device
func (ap *AudioPlayer) PlaySound(soundFilePath string) error {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	// Stop any currently playing sound
	ap.stopCurrentSound()

	// Build the command to play the audio file
	args := ap.buildPlayCommand(soundFilePath)
	if len(args) == 0 {
		return fmt.Errorf("no suitable audio player found")
	}

	// Start the audio playback process
	cmd := exec.Command(args[0], args[1:]...)
	if err := cmd.Start(); err != nil {
		ap.lastError = fmt.Errorf("failed to start audio playback with %s: %w", args[0], err)
		return ap.lastError
	}

	// Store the current process and file for stopping later
	ap.currentProcess = cmd
	ap.currentSoundFile = soundFilePath
	ap.playbackStarted = time.Now()
	ap.lastError = nil

	// Start a goroutine to wait for the process to complete
	go func() {
		err := cmd.Wait()
		ap.mutex.Lock()
		defer ap.mutex.Unlock()

		if ap.currentProcess == cmd {
			ap.currentProcess = nil
			ap.currentSoundFile = ""

			// Store any error that occurred during playback
			if err != nil {
				ap.lastError = fmt.Errorf("audio playback failed: %w", err)
			}
		}
	}()

	return nil
}

// StopCurrentSound stops any currently playing sound
func (ap *AudioPlayer) StopCurrentSound() error {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()
	return ap.stopCurrentSound()
}

// stopCurrentSound stops the current sound (internal, assumes mutex is held)
func (ap *AudioPlayer) stopCurrentSound() error {
	if ap.currentProcess != nil {
		// Try to terminate the process gracefully
		if err := ap.currentProcess.Process.Kill(); err != nil {
			return fmt.Errorf("failed to stop audio playback: %w", err)
		}
		ap.currentProcess = nil
		ap.currentSoundFile = ""
	}
	return nil
}

// GetCurrentSoundFile returns the currently playing sound file path
func (ap *AudioPlayer) GetCurrentSoundFile() string {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()
	return ap.currentSoundFile
}

// IsPlaying returns true if audio is currently playing
func (ap *AudioPlayer) IsPlaying() bool {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()
	return ap.currentProcess != nil
}

// GetPlaybackStatus returns detailed playback status
func (ap *AudioPlayer) GetPlaybackStatus() map[string]interface{} {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	status := map[string]interface{}{
		"isPlaying":    ap.currentProcess != nil,
		"currentSound": ap.currentSoundFile,
	}

	if ap.currentProcess != nil {
		status["playbackDuration"] = time.Since(ap.playbackStarted).Seconds()
	}

	if ap.lastError != nil {
		status["lastError"] = ap.lastError.Error()
	}

	return status
}

// GetLastError returns the last error that occurred
func (ap *AudioPlayer) GetLastError() error {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()
	return ap.lastError
}

// ClearLastError clears the last error
func (ap *AudioPlayer) ClearLastError() {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()
	ap.lastError = nil
}

// buildPlayCommand constructs the command to play audio based on available tools and configuration
func (ap *AudioPlayer) buildPlayCommand(soundFilePath string) []string {
	// Try different audio players in order of preference
	players := []struct {
		cmd  string
		args func(string) []string
	}{
		// aplay (ALSA player) - good for WAV files
		{
			cmd: "aplay",
			args: func(file string) []string {
				args := []string{}
				if ap.config.ALSADevice != "" && ap.config.ALSADevice != "default" {
					args = append(args, "-D", ap.config.ALSADevice)
				}
				if ap.config.ALSACardName != "" {
					args = append(args, "-D", fmt.Sprintf("hw:%s", ap.config.ALSACardName))
				}
				args = append(args, file)
				return args
			},
		},
		// mpg123 - good for MP3 files
		{
			cmd: "mpg123",
			args: func(file string) []string {
				args := []string{"-q"} // quiet mode
				if ap.config.ALSADevice != "" && ap.config.ALSADevice != "default" {
					args = append(args, "-a", ap.config.ALSADevice)
				}
				args = append(args, file)
				return args
			},
		},
		// ogg123 - good for OGG files
		{
			cmd: "ogg123",
			args: func(file string) []string {
				args := []string{"-q"} // quiet mode
				if ap.config.ALSADevice != "" && ap.config.ALSADevice != "default" {
					args = append(args, "-d", "alsa", "-o", fmt.Sprintf("dev:%s", ap.config.ALSADevice))
				}
				args = append(args, file)
				return args
			},
		},
		// ffplay (from ffmpeg) - supports many formats
		{
			cmd: "ffplay",
			args: func(file string) []string {
				args := []string{"-nodisp", "-autoexit", "-loglevel", "quiet"}
				if ap.config.ALSADevice != "" && ap.config.ALSADevice != "default" {
					args = append(args, "-f", "alsa", "-i", ap.config.ALSADevice)
				}
				args = append(args, file)
				return args
			},
		},
		// paplay (PulseAudio) - fallback if ALSA not available
		{
			cmd: "paplay",
			args: func(file string) []string {
				return []string{file}
			},
		},
		// Generic fallback using aplay without device specification
		{
			cmd: "aplay",
			args: func(file string) []string {
				return []string{file}
			},
		},
	}

	// Try each player until we find one that exists
	for _, player := range players {
		if _, err := exec.LookPath(player.cmd); err == nil {
			args := player.args(soundFilePath)
			return append([]string{player.cmd}, args...)
		}
	}

	return []string{}
}

// GetAudioPlayerInfo returns information about the available audio players
func (ap *AudioPlayer) GetAudioPlayerInfo() map[string]bool {
	players := []string{"aplay", "mpg123", "ogg123", "ffplay", "paplay"}
	info := make(map[string]bool)

	for _, player := range players {
		_, err := exec.LookPath(player)
		info[player] = err == nil
	}

	return info
}

// SetVolume sets the ALSA volume using amixer
func (ap *AudioPlayer) SetVolume(volume int) error {
	if volume < 0 || volume > 100 {
		return fmt.Errorf("volume must be between 0 and 100, got %d", volume)
	}

	// Try different amixer approaches
	commands := [][]string{
		// Try with configured device first
		{"amixer", "-D", ap.config.ALSADevice, "sset", "Master", fmt.Sprintf("%d%%", volume)},
		// Try with card name if configured
		{"amixer", "-c", ap.config.ALSACardName, "sset", "Master", fmt.Sprintf("%d%%", volume)},
		// Try default master control
		{"amixer", "sset", "Master", fmt.Sprintf("%d%%", volume)},
		// Try PCM control as fallback
		{"amixer", "sset", "PCM", fmt.Sprintf("%d%%", volume)},
	}

	var lastErr error
	for _, cmdArgs := range commands {
		// Skip if missing required parameters
		if (cmdArgs[1] == "-D" && ap.config.ALSADevice == "") ||
			(cmdArgs[1] == "-c" && ap.config.ALSACardName == "") {
			continue
		}

		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		if err := cmd.Run(); err == nil {
			return nil // Success
		} else {
			lastErr = err
		}
	}

	return fmt.Errorf("failed to set volume with amixer: %w", lastErr)
}

// GetVolume gets the current ALSA volume using amixer
func (ap *AudioPlayer) GetVolume() (int, error) {
	// Try different amixer approaches to get volume
	commands := [][]string{
		// Try with configured device first
		{"amixer", "-D", ap.config.ALSADevice, "sget", "Master"},
		// Try with card name if configured
		{"amixer", "-c", ap.config.ALSACardName, "sget", "Master"},
		// Try default master control
		{"amixer", "sget", "Master"},
		// Try PCM control as fallback
		{"amixer", "sget", "PCM"},
	}

	var lastErr error
	for _, cmdArgs := range commands {
		// Skip if missing required parameters
		if (len(cmdArgs) > 2 && cmdArgs[1] == "-D" && ap.config.ALSADevice == "") ||
			(len(cmdArgs) > 2 && cmdArgs[1] == "-c" && ap.config.ALSACardName == "") {
			continue
		}

		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		output, err := cmd.Output()
		if err != nil {
			lastErr = err
			continue
		}

		// Parse amixer output to extract volume percentage
		volume, err := ap.parseAmixerVolume(string(output))
		if err != nil {
			lastErr = err
			continue
		}

		return volume, nil
	}

	return 0, fmt.Errorf("failed to get volume with amixer: %w", lastErr)
}

// parseAmixerVolume parses amixer output to extract volume percentage
func (ap *AudioPlayer) parseAmixerVolume(output string) (int, error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Look for lines containing volume info like "[75%]"
		if strings.Contains(line, "[") && strings.Contains(line, "%]") {
			start := strings.Index(line, "[") + 1
			end := strings.Index(line, "%]")
			if start > 0 && end > start {
				volumeStr := line[start:end]
				if volume, err := strconv.Atoi(volumeStr); err == nil {
					return volume, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("could not parse volume from amixer output")
}
