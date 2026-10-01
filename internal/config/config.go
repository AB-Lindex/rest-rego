package config

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/AB-Lindex/rest-rego/internal/types"
	"github.com/alexflint/go-arg"
	"github.com/ninlil/envsubst"
)

// reservedMetricLabelNames are the fixed label names already used by rest-rego's HTTP metrics.
var reservedMetricLabelNames = map[string]bool{
	"method": true,
	"code":   true,
	"url":    true,
}

// metricLabelNameRE matches the Prometheus label-name grammar.
var metricLabelNameRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Fields is the configuration structure
type Fields struct {
	Verbose              bool     `arg:"-v,help:verbosity"`
	Debug                bool     `arg:"--debug" help:"print policy request and result"`
	PolicyDir            string   `arg:"-d,--directory,env:POLICY_DIR" default:"./policies" help:"directory containing policy files" placeholder:"DIR"`
	FilePattern          string   `arg:"--pattern,env:FILE_PATTERN" default:"*.rego" help:"pattern for policy files" placeholder:"PATTERN"`
	RequestRego          string   `arg:"-r,env:REQUEST" default:"request.rego" help:"policy for incoming requests" placeholder:"FILE"`
	ListenAddr           string   `arg:"-l,--listen,env:LISTEN_ADDR" default:":8181" help:"port for to listen on for proxy" placeholder:"ADDR"`
	MgmtAddr             string   `arg:"-m,--management,env:MGMT_ADDR" default:":8182" help:"port to listen on for management (probes)" placeholder:"ADDR"`
	AzureTenant          string   `arg:"-t,--azure-tenant,env:AZURE_TENANT" help:"azure tenant id" placeholder:"ID"`
	AuthHeader           string   `arg:"-a,--auth-header,env:AUTH_HEADER" default:"Authorization" placeholder:"HEADER"`
	AuthKind             string   `arg:"-k,--auth-kind,env:AUTH_KIND" default:"bearer" placeholder:"KIND"`
	BackendScheme        string   `arg:"-s,--backend-scheme,env:BACKEND_SCHEME" default:"http" help:"scheme for backend" placeholder:"SCHEME"`
	BackendHost          string   `arg:"-h,--backend-host,env:BACKEND_HOST" default:"localhost" help:"host for backend" placeholder:"HOST"`
	BackendPort          int      `arg:"-p,--backend-port,env:BACKEND_PORT" default:"8080" help:"port for backend" placeholder:"PORT"`
	WellKnownURL         []string `arg:"-w,--well-known,env:WELLKNOWN_OIDC" help:"well-known URL for JWK verifications" placeholder:"URL"`
	Audiences            []string `arg:"-u,--audience,env:JWT_AUDIENCES" help:"audience for JWT verification" placeholder:"AUDIENCE"`
	AudienceKey          string   `arg:"--audience-key,env:JWT_AUDIENCE_KEY" default:"aud" help:"claim key to use for audience check" placeholder:"KEY"`
	PermissiveAuth       bool     `arg:"--permissive-auth,env:PERMISSIVE_AUTH" default:"false" help:"allow invalid tokens to be treated as anonymous (default: false, strict mode)"`
	ResourceURL          string   `arg:"--resource-url,env:RESOURCE_URL" help:"externally-reachable URL of this protected resource (RFC 9728); required for JWT mode to serve metadata" placeholder:"URL"`
	ResourceMetadataPath string   `arg:"--resource-metadata-path,env:RESOURCE_METADATA_PATH" default:"/.well-known/oauth-protected-resource" help:"path this instance listens on (and advertises) for RFC 9728 metadata; empty keeps the RFC default" placeholder:"PATH"`
	ResourceScopes       []string `arg:"--resource-scopes,env:RESOURCE_SCOPES" help:"OAuth scope(s) this resource exposes, advertised as scopes_supported in RFC 9728 metadata (requires resource-url)" placeholder:"SCOPE"`
	BasicAuthFile        string   `arg:"--basic-auth-file,env:BASIC_AUTH_FILE" help:"path to Apache 2.4 htpasswd file (bcrypt only)" placeholder:"FILE"`
	NoAuth               bool     `arg:"--no-auth,env:NO_AUTH" default:"false" help:"disable authentication — policy is the sole access control (requires explicit opt-in)"`
	ExposeBlockedHeaders bool     `arg:"--expose-blocked-headers,env:EXPOSE_BLOCKED_HEADERS" default:"false" help:"expose X-Restrego-* headers to policy as blocked_headers (security: headers still removed from backend)"`
	EnvsubstPrefix       string   `arg:"--envsubst-prefix,env:ENVSUBST_PREFIX" default:"$" help:"prefix character for env var expansion in policies (one of: $ % & #)" placeholder:"CHAR"`
	EnvsubstWrapper      string   `arg:"--envsubst-wrapper,env:ENVSUBST_WRAPPER" default:"{" help:"wrapper character for env var expansion in policies (one of: { ( [ <)" placeholder:"CHAR"`
	URLMetricsLevel      int      `arg:"--url-metrics-level,env:URL_METRICS_LEVEL" default:"0" help:"level of URL detail to include in metrics (<0=full path, 0=none, >0=up to N segments)"`
	MetricLabels         []string `arg:"--metric-labels,env:METRIC_LABELS" help:"names of custom Prometheus labels populated by policy 'labels' results" placeholder:"NAME"`
	MetricLabelMaxLength int      `arg:"--metric-label-max-length,env:METRIC_LABEL_MAX_LENGTH" default:"20" help:"maximum length of a custom metric label value"`
	MetricLabelDefault   string   `arg:"--metric-label-default,env:METRIC_LABEL_DEFAULT" default:"-" help:"default value for a custom metric label when missing or empty"`

	// Timeout configuration for proxy server
	ReadHeaderTimeout time.Duration `arg:"--read-header-timeout,env:READ_HEADER_TIMEOUT" default:"10s" help:"timeout for reading request headers"`
	ReadTimeout       time.Duration `arg:"--read-timeout,env:READ_TIMEOUT" default:"30s" help:"timeout for reading entire request"`
	WriteTimeout      time.Duration `arg:"--write-timeout,env:WRITE_TIMEOUT" default:"90s" help:"timeout for writing response"`
	IdleTimeout       time.Duration `arg:"--idle-timeout,env:IDLE_TIMEOUT" default:"120s" help:"timeout for idle connections"`

	// Timeout configuration for backend communication
	BackendDialTimeout     time.Duration `arg:"--backend-dial-timeout,env:BACKEND_DIAL_TIMEOUT" default:"10s" help:"timeout for backend connection"`
	BackendResponseTimeout time.Duration `arg:"--backend-response-timeout,env:BACKEND_RESPONSE_TIMEOUT" default:"30s" help:"timeout for backend response headers"`
	BackendIdleConnTimeout time.Duration `arg:"--backend-idle-timeout,env:BACKEND_IDLE_TIMEOUT" default:"90s" help:"timeout for idle backend connections"`
}

func (f *Fields) Version() string {
	return types.Version()
}

// validateEnvsubst validates the envsubst prefix and wrapper characters.
func (f *Fields) validateEnvsubst() {
	if !envsubst.SetPrefix(f.EnvsubstPrefixRune()) {
		slog.Error("config: invalid envsubst-prefix", "value", f.EnvsubstPrefix, "valid", "$ % & #")
		os.Exit(1)
	}
	if !envsubst.SetWrapper(f.EnvsubstWrapperRune()) {
		slog.Error("config: invalid envsubst-wrapper", "value", f.EnvsubstWrapper, "valid", "{ ( [ <")
		os.Exit(1)
	}
}

// EnvsubstPrefixRune returns the envsubst prefix as a rune.
func (f *Fields) EnvsubstPrefixRune() rune { return rune(f.EnvsubstPrefix[0]) }

// EnvsubstWrapperRune returns the envsubst wrapper as a rune.
func (f *Fields) EnvsubstWrapperRune() rune { return rune(f.EnvsubstWrapper[0]) }

// validateMetricLabelsConfig checks the custom metric label configuration and returns
// an error describing the first problem found, or nil if the configuration is valid.
func (f *Fields) validateMetricLabelsConfig() error {
	if f.MetricLabelMaxLength <= 0 {
		return fmt.Errorf("metric-label-max-length must be > 0, got %d", f.MetricLabelMaxLength)
	}

	seen := make(map[string]bool, len(f.MetricLabels))
	for _, name := range f.MetricLabels {
		if !metricLabelNameRE.MatchString(name) {
			return fmt.Errorf("metric-labels: invalid label name %q (must match %s)", name, metricLabelNameRE.String())
		}
		if reservedMetricLabelNames[name] {
			return fmt.Errorf("metric-labels: %q collides with a reserved label name", name)
		}
		if seen[name] {
			return fmt.Errorf("metric-labels: duplicate label name %q", name)
		}
		seen[name] = true
	}

	return nil
}

// validateMetricLabels validates the custom metric label configuration, exiting the
// process on failure.
func (f *Fields) validateMetricLabels() {
	if err := f.validateMetricLabelsConfig(); err != nil {
		slog.Error("config: invalid metric-labels configuration", "error", err)
		os.Exit(1)
	}
}

// validateResourceMetadataConfig validates and normalizes RFC 9728 metadata config,
// returning an error describing the first problem found, or nil if valid.
func (f *Fields) validateResourceMetadataConfig() error {
	if f.ResourceMetadataPath == "" {
		f.ResourceMetadataPath = "/.well-known/oauth-protected-resource"
	} else {
		// normalize to a leading slash and no trailing slash so it matches r.URL.Path
		f.ResourceMetadataPath = "/" + strings.Trim(f.ResourceMetadataPath, "/")
	}
	if f.ResourceURL == "" && len(f.ResourceScopes) > 0 {
		slog.Warn("config: resource-scopes configured but resource-url is not set — ignored, RFC 9728 metadata endpoint disabled")
	}
	if f.ResourceURL == "" {
		if len(f.WellKnownURL) > 0 {
			slog.Warn("config: JWT (OIDC) mode active but resource-url is not set — RFC 9728 metadata endpoint disabled")
		}
		return nil
	}
	if len(f.WellKnownURL) == 0 {
		slog.Warn("config: resource-url is configured but JWT (OIDC) auth mode is not active — ignoring")
		return nil
	}
	u, err := url.Parse(f.ResourceURL)
	if err != nil || !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("resource-url must be an absolute http(s) URL with a host, got %q", f.ResourceURL)
	}
	return nil
}

// validateResourceMetadata validates the RFC 9728 metadata configuration, exiting the
// process on failure.
func (f *Fields) validateResourceMetadata() {
	if err := f.validateResourceMetadataConfig(); err != nil {
		slog.Error("config: invalid resource-url configuration", "error", err)
		os.Exit(1)
	}
}

// validateTimeouts validates timeout configuration values
func (f *Fields) validateTimeouts() {
	// Minimum timeout: 1s (prevents accidental misconfiguration)
	// Maximum timeout: 10m (generous but prevents indefinite hangs)
	const (
		minTimeout = 1 * time.Second
		maxTimeout = 10 * time.Minute
	)

	timeouts := map[string]*time.Duration{
		"read-header-timeout":      &f.ReadHeaderTimeout,
		"read-timeout":             &f.ReadTimeout,
		"write-timeout":            &f.WriteTimeout,
		"idle-timeout":             &f.IdleTimeout,
		"backend-dial-timeout":     &f.BackendDialTimeout,
		"backend-response-timeout": &f.BackendResponseTimeout,
		"backend-idle-timeout":     &f.BackendIdleConnTimeout,
	}

	for name, timeout := range timeouts {
		if *timeout < minTimeout {
			slog.Error("config: timeout too short",
				"timeout", name,
				"value", *timeout,
				"minimum", minTimeout)
			os.Exit(1)
		}
		if *timeout > maxTimeout {
			slog.Error("config: timeout too long",
				"timeout", name,
				"value", *timeout,
				"maximum", maxTimeout)
			os.Exit(1)
		}
	}

	// Logical validation: ReadTimeout should be >= ReadHeaderTimeout
	if f.ReadTimeout < f.ReadHeaderTimeout {
		slog.Error("config: read-timeout must be >= read-header-timeout",
			"read-timeout", f.ReadTimeout,
			"read-header-timeout", f.ReadHeaderTimeout)
		os.Exit(1)
	}

	// Log timeout configuration at debug level
	slog.Debug("config: timeout configuration validated",
		"read-header", f.ReadHeaderTimeout,
		"read", f.ReadTimeout,
		"write", f.WriteTimeout,
		"idle", f.IdleTimeout,
		"backend-dial", f.BackendDialTimeout,
		"backend-response", f.BackendResponseTimeout,
		"backend-idle", f.BackendIdleConnTimeout)
}

// New creates a new instance of the configuration
func New() *Fields {
	f := &Fields{}
	arg.MustParse(f)
	if f.Verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		slog.Debug("config: verbosity enabled")
	}

	// Validate envsubst configuration
	f.validateEnvsubst()

	// Validate timeout configuration
	f.validateTimeouts()

	// Validate custom metric label configuration
	f.validateMetricLabels()

	authCount := 0
	if f.AzureTenant != "" {
		authCount++
	}
	if len(f.WellKnownURL) > 0 {
		authCount++
	}
	if f.BasicAuthFile != "" {
		authCount++
	}
	if f.NoAuth {
		authCount++
	}
	if authCount > 1 {
		slog.Error("config: only one auth-provider may be configured (AZURE_TENANT, WELLKNOWN_OIDC, BASIC_AUTH_FILE) or using NO_AUTH mode")
		os.Exit(1)
	}
	if len(f.WellKnownURL) > 0 && len(f.Audiences) == 0 {
		slog.Error("config: audiences must be provided when using well-known")
		os.Exit(1)
	}
	f.validateResourceMetadata()
	if len(f.AuthHeader) == 0 {
		slog.Error("config: auth-header must be provided")
		os.Exit(1)
	}
	// need to make sure the auth-header is in proper canonical format
	f.AuthHeader = http.CanonicalHeaderKey(f.AuthHeader)

	// Log authentication mode
	if f.PermissiveAuth {
		slog.Warn("config: permissive authentication mode enabled - invalid tokens will be treated as anonymous")
	} else {
		slog.Info("config: strict authentication mode - invalid tokens will be rejected")
	}

	if f.URLMetricsLevel < 0 {
		slog.Warn("config: url-metrics-level is negative — full request paths will be used as Prometheus url labels, which may cause unbounded cardinality")
	}

	if len(f.MetricLabels) > 0 {
		slog.Warn("config: metric-labels is configured — custom Prometheus labels are populated from policy results, which may cause unbounded cardinality", "labels", f.MetricLabels)
	}

	return f
}
