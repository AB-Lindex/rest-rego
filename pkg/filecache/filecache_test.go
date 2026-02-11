package filecache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCache_shouldProcess(t *testing.T) {
	// Setup: create temporary directory with various file types
	tmpDir := t.TempDir()

	// Create regular files
	regularFile := filepath.Join(tmpDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create regular file: %v", err)
	}

	hiddenFile := filepath.Join(tmpDir, ".hidden.txt")
	if err := os.WriteFile(hiddenFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create hidden file: %v", err)
	}

	noMatchFile := filepath.Join(tmpDir, "nomatch.log")
	if err := os.WriteFile(noMatchFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create nomatch file: %v", err)
	}

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir.txt")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// Create symlink
	symlinkTarget := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(symlinkTarget, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create symlink target: %v", err)
	}
	symlink := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(symlinkTarget, symlink); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Create ConfigMap-style hidden files
	configMapFile := filepath.Join(tmpDir, "..data")
	if err := os.WriteFile(configMapFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create configmap file: %v", err)
	}

	tests := []struct {
		name      string // description of this test case
		folder    string
		pattern   string
		fullPath  string
		inputname string
		want      bool
	}{
		{
			name:      "regular file matching pattern",
			folder:    tmpDir,
			pattern:   "*.txt",
			fullPath:  regularFile,
			inputname: "regular.txt",
			want:      true,
		},
		{
			name:      "hidden file matching pattern",
			folder:    tmpDir,
			pattern:   "*.txt",
			fullPath:  hiddenFile,
			inputname: ".hidden.txt",
			want:      false, // filtered out by hidden file check
		},
		{
			name:      "file not matching pattern",
			folder:    tmpDir,
			pattern:   "*.txt",
			fullPath:  noMatchFile,
			inputname: "nomatch.log",
			want:      false, // pattern doesn't match
		},
		{
			name:      "directory with matching name",
			folder:    tmpDir,
			pattern:   "*.txt",
			fullPath:  subDir,
			inputname: "subdir.txt",
			want:      false, // directories are not regular files
		},
		{
			name:      "symlink with matching name",
			folder:    tmpDir,
			pattern:   "*.txt",
			fullPath:  symlink,
			inputname: "link.txt",
			want:      false, // symlinks are not regular files (Lstat returns symlink info)
		},
		{
			name:      "kubernetes configmap hidden file",
			folder:    tmpDir,
			pattern:   "*",
			fullPath:  configMapFile,
			inputname: "..data",
			want:      false, // starts with dot
		},
		{
			name:      "non-existent file matching pattern",
			folder:    tmpDir,
			pattern:   "*.txt",
			fullPath:  filepath.Join(tmpDir, "nonexistent.txt"),
			inputname: "nonexistent.txt",
			want:      true, // stat fails, returns true for downstream handling
		},
		{
			name:      "wildcard pattern matches all non-hidden files",
			folder:    tmpDir,
			pattern:   "*",
			fullPath:  regularFile,
			inputname: "regular.txt",
			want:      true,
		},
		{
			name:      "specific filename pattern",
			folder:    tmpDir,
			pattern:   "regular.txt",
			fullPath:  regularFile,
			inputname: "regular.txt",
			want:      true,
		},
		{
			name:      "multiple extensions pattern",
			folder:    tmpDir,
			pattern:   "*.txt",
			fullPath:  regularFile,
			inputname: "regular.txt",
			want:      true,
		},
		{
			name:      "dot prefix file (current directory marker style)",
			folder:    tmpDir,
			pattern:   "*",
			fullPath:  filepath.Join(tmpDir, ".config"),
			inputname: ".config",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(tt.folder, tt.pattern)
			if err != nil {
				t.Fatalf("could not construct receiver type: %v", err)
			}
			defer c.Close()

			got := c.shouldProcess(tt.fullPath, tt.inputname)
			if got != tt.want {
				t.Errorf("shouldProcess() = %v, want %v", got, tt.want)
			}
		})
	}
}
