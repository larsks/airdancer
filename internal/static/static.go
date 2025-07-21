package static

import (
	"embed"
	"io/fs"
)

// Static assets embedded at build time
//
//go:embed *.css *.js *.html
var assets embed.FS

// GetAssets returns the embedded filesystem containing static assets
func GetAssets() fs.FS {
	return assets
}

// GetCSS returns the contents of the common CSS file
func GetCSS() ([]byte, error) {
	return assets.ReadFile("styles.css")
}

// GetJS returns the contents of the common JavaScript file
func GetJS() ([]byte, error) {
	return assets.ReadFile("common.js")
}

// GetSwitchControllerJS returns the contents of the switch controller JavaScript file
func GetSwitchControllerJS() ([]byte, error) {
	return assets.ReadFile("switch-controller.js")
}

// GetSoundboardControllerJS returns the contents of the soundboard controller JavaScript file
func GetSoundboardControllerJS() ([]byte, error) {
	return assets.ReadFile("soundboard-controller.js")
}

// GetSoundboardCSS returns the contents of the soundboard-specific CSS file
func GetSoundboardCSS() ([]byte, error) {
	return assets.ReadFile("soundboard.css")
}

// GetSwitchControlContent returns the HTML content for the switch control interface
func GetSwitchControlContent() ([]byte, error) {
	return assets.ReadFile("switch-control-content.html")
}

// GetSoundboardContent returns the HTML content for the soundboard interface
func GetSoundboardContent() ([]byte, error) {
	return assets.ReadFile("soundboard-content.html")
}
