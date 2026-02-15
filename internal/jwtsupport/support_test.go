package jwtsupport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsFileURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"file with single slash", "file:/path", true},
		{"file with triple slash", "file:///path", true},
		{"file with localhost", "file://localhost/path", true},
		{"https URL", "https://example.com", false},
		{"http URL", "http://example.com", false},
		{"empty string", "", false},
		{"ftp URL", "ftp://example.com", false},
		{"relative file path", "file:config/test.json", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isFileURL(tt.url)
			if result != tt.expected {
				t.Errorf("isFileURL(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestFileURLToPath(t *testing.T) {
	tests := []struct {
		name        string
		fileURL     string
		expected    string
		shouldError bool
	}{
		// Valid absolute paths
		{"triple slash absolute", "file:///tmp/a.json", "/tmp/a.json", false},
		{"single slash absolute", "file:/tmp/a.json", "/tmp/a.json", false},
		{"localhost absolute", "file://localhost/tmp/a.json", "/tmp/a.json", false},

		// Valid relative paths
		{"relative path", "file:config/test.json", "config/test.json", false},
		{"relative path with dot", "file:./config/test.json", "config/test.json", false},

		// Path traversal attacks (should error)
		{"path traversal parent", "file:../etc/passwd", "", true},
		{"path traversal nested", "file:config/../../etc/passwd", "", true},
		{"path traversal absolute", "file:///config/../../../etc/passwd", "", true},
		{"path traversal double dot start", "file:..///etc/passwd", "", true},

		// Malformed URLs
		{"malformed URL", "file://[::1", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fileURLToPath(tt.fileURL)

			if tt.shouldError {
				if err == nil {
					t.Errorf("fileURLToPath(%q) expected error but got none, result: %q", tt.fileURL, result)
				}
			} else {
				if err != nil {
					t.Errorf("fileURLToPath(%q) unexpected error: %v", tt.fileURL, err)
				}
				if result != tt.expected {
					t.Errorf("fileURLToPath(%q) = %q, expected %q", tt.fileURL, result, tt.expected)
				}
			}
		})
	}
}

func TestReadFileURL(t *testing.T) {
	// Create a temporary file with known content
	tmpDir := t.TempDir()
	testContent := []byte(`{"test": "content"}`)
	testFile := filepath.Join(tmpDir, "test.json")

	err := os.WriteFile(testFile, testContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		fileURL     string
		expected    []byte
		shouldError bool
	}{
		{
			name:        "read existing file with absolute path",
			fileURL:     "file://" + testFile,
			expected:    testContent,
			shouldError: false,
		},
		{
			name:        "read non-existent file",
			fileURL:     "file:///nonexistent/file.json",
			expected:    nil,
			shouldError: true,
		},
		{
			name:        "path traversal attempt",
			fileURL:     "file:../../../etc/passwd",
			expected:    nil,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := readFileURL(tt.fileURL)

			if tt.shouldError {
				if err == nil {
					t.Errorf("readFileURL(%q) expected error but got none", tt.fileURL)
				}
			} else {
				if err != nil {
					t.Errorf("readFileURL(%q) unexpected error: %v", tt.fileURL, err)
				}
				if string(result) != string(tt.expected) {
					t.Errorf("readFileURL(%q) = %q, expected %q", tt.fileURL, result, tt.expected)
				}
			}
		})
	}

	// Test relative path resolution
	t.Run("relative path resolution", func(t *testing.T) {
		// Create a test file in the current directory
		relTestFile := "test-relative-jwks.json"
		relContent := []byte(`{"relative": "test"}`)

		err := os.WriteFile(relTestFile, relContent, 0644)
		if err != nil {
			t.Fatalf("Failed to create relative test file: %v", err)
		}
		defer os.Remove(relTestFile)

		result, err := readFileURL("file:" + relTestFile)
		if err != nil {
			t.Errorf("readFileURL with relative path failed: %v", err)
		}
		if string(result) != string(relContent) {
			t.Errorf("readFileURL relative path content = %q, expected %q", result, relContent)
		}
	})
}

func TestSourceType(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"file URL", "file:///x", "file"},
		{"https URL", "https://x", "http"},
		{"http URL", "http://example.com", "http"},
		{"file relative", "file:config/test.json", "file"},
		{"empty string", "", "http"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sourceType(tt.url)
			if result != tt.expected {
				t.Errorf("sourceType(%q) = %q, expected %q", tt.url, result, tt.expected)
			}
		})
	}
}
