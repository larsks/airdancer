package soundboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSoundManager(t *testing.T) {
	sm := NewSoundManager("/test/path")

	if sm.soundDirectory != "/test/path" {
		t.Errorf("expected sound directory '/test/path', got %q", sm.soundDirectory)
	}

	if len(sm.sounds) != 0 {
		t.Errorf("expected empty sounds slice, got %d items", len(sm.sounds))
	}
}

func TestIsSoundFile(t *testing.T) {
	sm := NewSoundManager(".")

	testCases := []struct {
		ext      string
		expected bool
	}{
		{".mp3", true},
		{".wav", true},
		{".ogg", true},
		{".flac", true},
		{".m4a", true},
		{".aac", true},
		{".txt", false},
		{".json", false},
		{".jpg", false},
		{"", false},
	}

	for _, tc := range testCases {
		result := sm.isSoundFile(tc.ext)
		if result != tc.expected {
			t.Errorf("isSoundFile(%q) = %v, expected %v", tc.ext, result, tc.expected)
		}
	}
}

func TestGetFileNameWithoutExt(t *testing.T) {
	sm := NewSoundManager(".")

	testCases := []struct {
		filename string
		expected string
	}{
		{"test.mp3", "test"},
		{"sound.wav", "sound"},
		{"music.ogg", "music"},
		{"no_extension", "no_extension"},
		{"multiple.dots.mp3", "multiple.dots"},
		{"", ""},
	}

	for _, tc := range testCases {
		result := sm.getFileNameWithoutExt(tc.filename)
		if result != tc.expected {
			t.Errorf("getFileNameWithoutExt(%q) = %q, expected %q", tc.filename, result, tc.expected)
		}
	}
}

func TestGetSoundsPage(t *testing.T) {
	sm := NewSoundManager(".")

	// Add some test sounds
	for i := 0; i < 25; i++ {
		sound := Sound{
			FileName:    "test" + string(rune('0'+i)) + ".mp3",
			DisplayName: "Test Sound " + string(rune('0'+i)),
		}
		sm.sounds = append(sm.sounds, sound)
	}

	// Test first page
	sounds, totalPages, err := sm.GetSoundsPage(1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sounds) != 10 {
		t.Errorf("expected 10 sounds on first page, got %d", len(sounds))
	}

	if totalPages != 3 {
		t.Errorf("expected 3 total pages, got %d", totalPages)
	}

	// Test last page
	sounds, totalPages, err = sm.GetSoundsPage(3, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sounds) != 5 {
		t.Errorf("expected 5 sounds on last page, got %d", len(sounds))
	}

	// Test out of bounds page
	sounds, totalPages, err = sm.GetSoundsPage(5, 10)
	if err == nil {
		t.Error("expected error for out of bounds page")
	}

	// Test invalid page number
	sounds, totalPages, err = sm.GetSoundsPage(0, 10)
	if err != nil {
		t.Fatalf("unexpected error for page 0: %v", err)
	}

	if len(sounds) != 10 {
		t.Errorf("expected 10 sounds for page 0 (normalized to 1), got %d", len(sounds))
	}
}

func TestLoadSoundMetadata(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "soundboard_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sm := NewSoundManager(tempDir)

	// Test sound without metadata
	sound := Sound{
		FileName: "test.mp3",
		FilePath: filepath.Join(tempDir, "test.mp3"),
	}

	err = sm.loadSoundMetadata(&sound)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sound.DisplayName != "test" {
		t.Errorf("expected display name 'test', got %q", sound.DisplayName)
	}

	// Test sound with metadata
	metadata := SoundMetadata{
		DisplayName: "Custom Display Name",
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}

	metadataPath := filepath.Join(tempDir, "test.json")
	err = os.WriteFile(metadataPath, metadataBytes, 0644)
	if err != nil {
		t.Fatalf("failed to write metadata file: %v", err)
	}

	sound2 := Sound{
		FileName: "test.mp3",
		FilePath: filepath.Join(tempDir, "test.mp3"),
	}

	err = sm.loadSoundMetadata(&sound2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sound2.DisplayName != "Custom Display Name" {
		t.Errorf("expected display name 'Custom Display Name', got %q", sound2.DisplayName)
	}
}

func TestLoadSounds(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "soundboard_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFiles := []string{"sound1.mp3", "sound2.wav", "not_sound.txt", "sound3.ogg"}
	for _, filename := range testFiles {
		filepath := filepath.Join(tempDir, filename)
		err := os.WriteFile(filepath, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("failed to create test file %s: %v", filename, err)
		}
	}

	// Create metadata for sound1
	metadata := SoundMetadata{
		DisplayName: "First Sound",
	}
	metadataBytes, _ := json.Marshal(metadata)
	metadataPath := filepath.Join(tempDir, "sound1.json")
	os.WriteFile(metadataPath, metadataBytes, 0644)

	sm := NewSoundManager(tempDir)
	err = sm.LoadSounds()
	if err != nil {
		t.Fatalf("failed to load sounds: %v", err)
	}

	sounds := sm.GetSounds()
	if len(sounds) != 3 {
		t.Errorf("expected 3 sound files, got %d", len(sounds))
	}

	// Check that sound1 has custom display name
	found := false
	for _, sound := range sounds {
		if sound.FileName == "sound1.mp3" {
			if sound.DisplayName != "First Sound" {
				t.Errorf("expected display name 'First Sound', got %q", sound.DisplayName)
			}
			found = true
			break
		}
	}

	if !found {
		t.Error("sound1.mp3 not found in loaded sounds")
	}
}

func TestLoadSoundsNonExistentDir(t *testing.T) {
	sm := NewSoundManager("/non/existent/directory")
	err := sm.LoadSounds()
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}
