package config

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/AB-Lindex/rest-rego/internal/types"
	"github.com/alexflint/go-arg"
)

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
	ExposeBlockedHeaders bool     `arg:"--expose-blocked-headers,env:EXPOSE_BLOCKED_HEADERS" default:"false" help:"expose X-Restrego-* headers to policy as blocked_headers (security: headers still removed from backend)"`

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

	// Validate timeout configuration
	f.validateTimeouts()

	if f.AzureTenant != "" && len(f.WellKnownURL) > 0 {
		slog.Error("config: only one auth-provider can be used (azure or well-known)")
		os.Exit(1)
	}
	if len(f.WellKnownURL) > 0 && len(f.Audiences) == 0 {
		slog.Error("config: audiences must be provided when using well-known")
		os.Exit(1)
	}
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

	return f
}
