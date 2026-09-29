package config

import (
	"testing"

	"github.com/alexflint/go-arg"
)

func TestValidateMetricLabelsConfig(t *testing.T) {
	tests := []struct {
		name    string
		fields  Fields
		wantErr bool
	}{
		{
			name:    "valid label names",
			fields:  Fields{MetricLabels: []string{"client_version", "tenant"}, MetricLabelMaxLength: 20},
			wantErr: false,
		},
		{
			name:    "no labels configured",
			fields:  Fields{MetricLabelMaxLength: 20},
			wantErr: false,
		},
		{
			name:    "invalid grammar - starts with digit",
			fields:  Fields{MetricLabels: []string{"1bad"}, MetricLabelMaxLength: 20},
			wantErr: true,
		},
		{
			name:    "invalid grammar - contains hyphen",
			fields:  Fields{MetricLabels: []string{"bad-name"}, MetricLabelMaxLength: 20},
			wantErr: true,
		},
		{
			name:    "reserved name collision - method",
			fields:  Fields{MetricLabels: []string{"method"}, MetricLabelMaxLength: 20},
			wantErr: true,
		},
		{
			name:    "reserved name collision - code",
			fields:  Fields{MetricLabels: []string{"code"}, MetricLabelMaxLength: 20},
			wantErr: true,
		},
		{
			name:    "reserved name collision - url",
			fields:  Fields{MetricLabels: []string{"url"}, MetricLabelMaxLength: 20},
			wantErr: true,
		},
		{
			name:    "duplicate names",
			fields:  Fields{MetricLabels: []string{"tenant", "tenant"}, MetricLabelMaxLength: 20},
			wantErr: true,
		},
		{
			name:    "max length zero",
			fields:  Fields{MetricLabelMaxLength: 0},
			wantErr: true,
		},
		{
			name:    "max length negative",
			fields:  Fields{MetricLabelMaxLength: -1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fields.validateMetricLabelsConfig()
			if tt.wantErr && err == nil {
				t.Errorf("expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestMetricLabelsDefaults(t *testing.T) {
	f := &Fields{}
	p, err := arg.NewParser(arg.Config{}, f)
	if err != nil {
		t.Fatalf("arg.NewParser: %v", err)
	}
	if err := p.Parse(nil); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if f.MetricLabelMaxLength != 20 {
		t.Errorf("expected default MetricLabelMaxLength 20, got %d", f.MetricLabelMaxLength)
	}
	if f.MetricLabelDefault != "-" {
		t.Errorf("expected default MetricLabelDefault %q, got %q", "-", f.MetricLabelDefault)
	}
	if len(f.MetricLabels) != 0 {
		t.Errorf("expected no default MetricLabels, got %v", f.MetricLabels)
	}
}

func TestValidateResourceMetadataConfig(t *testing.T) {
	tests := []struct {
		name         string
		fields       Fields
		wantErr      bool
		wantPath     string
		wantURLUnset bool
	}{
		{
			name:     "empty metadata path defaults",
			fields:   Fields{},
			wantErr:  false,
			wantPath: "/.well-known/oauth-protected-resource",
		},
		{
			name:         "resource-url set without JWT mode warns, no error",
			fields:       Fields{ResourceURL: "https://host/mcp"},
			wantErr:      false,
			wantPath:     "/.well-known/oauth-protected-resource",
			wantURLUnset: false,
		},
		{
			name:     "JWT mode active without resource-url warns, no error",
			fields:   Fields{WellKnownURL: []string{"https://idp/.well-known/openid-configuration"}},
			wantErr:  false,
			wantPath: "/.well-known/oauth-protected-resource",
		},
		{
			name: "JWT mode with valid absolute resource-url",
			fields: Fields{
				WellKnownURL: []string{"https://idp/.well-known/openid-configuration"},
				ResourceURL:  "https://host/mcp",
			},
			wantErr:  false,
			wantPath: "/.well-known/oauth-protected-resource",
		},
		{
			name: "JWT mode with malformed resource-url errors",
			fields: Fields{
				WellKnownURL: []string{"https://idp/.well-known/openid-configuration"},
				ResourceURL:  "not-a-url",
			},
			wantErr:  true,
			wantPath: "/.well-known/oauth-protected-resource",
		},
		{
			name: "JWT mode with relative resource-url errors",
			fields: Fields{
				WellKnownURL: []string{"https://idp/.well-known/openid-configuration"},
				ResourceURL:  "/mcp",
			},
			wantErr:  true,
			wantPath: "/.well-known/oauth-protected-resource",
		},
		{
			name: "custom metadata path is preserved",
			fields: Fields{
				ResourceMetadataPath: "/oauth/metadata",
			},
			wantErr:  false,
			wantPath: "/oauth/metadata",
		},
		{
			name:     "resource-scopes set without resource-url warns, no error",
			fields:   Fields{ResourceScopes: []string{"https://host/mcp/access_as_user"}},
			wantErr:  false,
			wantPath: "/.well-known/oauth-protected-resource",
		},
		{
			name: "JWT mode with valid resource-url and resource-scopes",
			fields: Fields{
				WellKnownURL:   []string{"https://idp/.well-known/openid-configuration"},
				ResourceURL:    "https://host/mcp",
				ResourceScopes: []string{"https://host/mcp/access_as_user"},
			},
			wantErr:  false,
			wantPath: "/.well-known/oauth-protected-resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.fields
			err := f.validateResourceMetadataConfig()
			if tt.wantErr && err == nil {
				t.Errorf("expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if f.ResourceMetadataPath != tt.wantPath {
				t.Errorf("expected ResourceMetadataPath %q, got %q", tt.wantPath, f.ResourceMetadataPath)
			}
		})
	}
}
