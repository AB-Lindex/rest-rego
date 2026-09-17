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
