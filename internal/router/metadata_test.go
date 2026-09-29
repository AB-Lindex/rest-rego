package router

import "testing"

func TestBuildWellKnownURL(t *testing.T) {
	tests := []struct {
		name         string
		resourceURL  string
		metadataPath string
		want         string
	}{
		{
			name:         "bare origin with default path",
			resourceURL:  "https://host",
			metadataPath: "/.well-known/oauth-protected-resource",
			want:         "https://host/.well-known/oauth-protected-resource",
		},
		{
			name:         "path-suffixed resource with default path",
			resourceURL:  "https://host/mcp",
			metadataPath: "/.well-known/oauth-protected-resource",
			want:         "https://host/.well-known/oauth-protected-resource/mcp",
		},
		{
			name:         "path-suffixed resource with custom metadata path",
			resourceURL:  "https://gateway.example.com/mcp-server",
			metadataPath: "/oauth/metadata",
			want:         "https://gateway.example.com/oauth/metadata/mcp-server",
		},
		{
			name:         "trailing slash on resource URL is normalized",
			resourceURL:  "https://host/mcp/",
			metadataPath: "/.well-known/oauth-protected-resource",
			want:         "https://host/.well-known/oauth-protected-resource/mcp",
		},
		{
			name:         "metadata path without leading slash is normalized",
			resourceURL:  "https://host/mcp",
			metadataPath: "oauth/metadata",
			want:         "https://host/oauth/metadata/mcp",
		},
		{
			name:         "metadata path with trailing slash is normalized",
			resourceURL:  "https://host/mcp",
			metadataPath: "/oauth/metadata/",
			want:         "https://host/oauth/metadata/mcp",
		},
		{
			name:         "bare origin with trailing slash",
			resourceURL:  "https://host/",
			metadataPath: "/.well-known/oauth-protected-resource",
			want:         "https://host/.well-known/oauth-protected-resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildWellKnownURL(tt.resourceURL, tt.metadataPath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("buildWellKnownURL(%q, %q) = %q, want %q", tt.resourceURL, tt.metadataPath, got, tt.want)
			}
		})
	}
}
