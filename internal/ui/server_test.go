package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUIServer(t *testing.T) {
	// Create a UI server with a test API URL
	apiURL := "http://test-api:8080"
	cfg := &Config{
		ListenAddress: "localhost",
		ListenPort:    8081,
		APIBaseURL:    apiURL,
	}
	server := NewUIServer(cfg)

	// Test the index handler
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	body := w.Body.String()

	// Check that the API URL was properly injected
	if !strings.Contains(body, apiURL) {
		t.Error("API URL was not properly injected into the HTML")
	}

	// Check that the HTML contains expected elements
	expectedElements := []string{
		"<title>Airdancer Switch Control</title>",
		"class=\"switches-grid\"",
		"API_BASE_URL =",
	}

	for _, element := range expectedElements {
		if !strings.Contains(body, element) {
			t.Errorf("expected HTML to contain %q, but it didn't", element)
		}
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected content type to contain text/html, got %q", contentType)
	}
}

func TestStaticFileServing(t *testing.T) {
	cfg := &Config{
		ListenAddress: "localhost",
		ListenPort:    8081,
		APIBaseURL:    "http://localhost:8080",
	}
	server := NewUIServer(cfg)

	// Test that the static file route is set up (even though we don't have other static files)
	req := httptest.NewRequest("GET", "/static/nonexistent.css", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	// Should return 404 for non-existent files, not a server error
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d for non-existent static file, got %d", http.StatusNotFound, w.Code)
	}
}
