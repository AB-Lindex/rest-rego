package jwtsupport

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// isFileURL returns true if the given URL string starts with "file:" scheme.
func isFileURL(urlStr string) bool {
	return strings.HasPrefix(urlStr, "file:")
}

// fileURLToPath converts a file:// URL to a file system path.
// It supports formats like file:///path, file:/path, file://localhost/path.
// Returns an error for malformed URLs or paths containing ".." components (path traversal prevention).
func fileURLToPath(fileURL string) (string, error) {
	parsed, err := url.Parse(fileURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse file URL: %w", err)
	}

	// Extract the path from the URL
	// For file:///path, parsed.Path = /path
	// For file://localhost/path, parsed.Path = /path  
	// For file:/path, parsed.Path = /path
	// For file:config/test.json, parsed.Opaque = config/test.json (opaque part, no Path)
	path := parsed.Path
	if path == "" && parsed.Opaque != "" {
		// Handle opaque URIs like file:config/test.json
		path = parsed.Opaque
	}

	// Path traversal prevention: check BEFORE cleaning
	// Check for ".." components in the original path
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path traversal not allowed")
	}

	// Clean the path to normalize it
	cleanPath := filepath.Clean(path)

	return cleanPath, nil
}

// readFileURL reads the content of a file specified by a file:// URL.
// Returns an error if the file cannot be read or the URL is invalid.
func readFileURL(fileURL string) ([]byte, error) {
	path, err := fileURLToPath(fileURL)
	if err != nil {
		return nil, fmt.Errorf("invalid file URL %q: %w", fileURL, err)
	}

	slog.Debug("jwtsupport: reading file", "url", fileURL, "path", path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file from URL %q (path: %s): %w", fileURL, path, err)
	}

	return data, nil
}

// sourceType returns "file" if the URL is a file:// URL, otherwise "http".
// Used for logging and error messages.
func sourceType(urlStr string) string {
	if isFileURL(urlStr) {
		return "file"
	}
	return "http"
}
