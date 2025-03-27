package config

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/AB-Lindex/rest-rego/internal/types"
	"github.com/alexflint/go-arg"
)

// Fields is the configuration structure
type Fields struct {
	Verbose       bool     `arg:"-v,help:verbosity"`
	Debug         bool     `arg:"--debug" help:"print policy request and result"`
	PolicyDir     string   `arg:"-d,--directory,env:POLICY_DIR" default:"./policies" help:"directory containing policy files" placeholder:"DIR"`
	FilePattern   string   `arg:"--pattern,env:FILE_PATTERN" default:"*.rego" help:"pattern for policy files" placeholder:"PATTERN"`
	RequestRego   string   `arg:"-r,env:REQUEST" default:"request.rego" help:"policy for incoming requests" placeholder:"FILE"`
	ListenAddr    string   `arg:"-l,--listen,env:LISTEN_ADDR" default:":8181" help:"port for to listen on for proxy" placeholder:"ADDR"`
	MgmtAddr      string   `arg:"-m,--management,env:MGMT_ADDR" default:":8182" help:"port to listen on for management (probes)" placeholder:"ADDR"`
	AzureTenant   string   `arg:"-t,--azure-tenant,env:AZURE_TENANT" help:"azure tenant id" placeholder:"ID"`
	AuthHeader    string   `arg:"-a,--auth-header,env:AUTH_HEADER" default:"Authorization" placeholder:"HEADER"`
	BackendScheme string   `arg:"-s,--backend-scheme,env:BACKEND_SCHEME" default:"http" help:"scheme for backend" placeholder:"SCHEME"`
	BackendHost   string   `arg:"-h,--backend-host,env:BACKEND_HOST" default:"localhost" help:"host for backend" placeholder:"HOST"`
	BackendPort   int      `arg:"-p,--backend-port,env:BACKEND_PORT" default:"8080" help:"port for backend" placeholder:"PORT"`
	WellKnownURL  []string `arg:"-w,--well-known,env:WELLKNOWN_OIDC" help:"well-known URL for JWK verifications" placeholder:"URL"`
	Audiences     []string `arg:"-u,--audience,env:JWT_AUDIENCES" help:"audience for JWT verification" placeholder:"AUDIENCE"`
}

func (f *Fields) Version() string {
	return types.Version()
}

// New creates a new instance of the configuration
func New() *Fields {
	f := &Fields{}
	arg.MustParse(f)
	if f.Verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		slog.Debug("config: verbosity enabled")
	}

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
	return f
}
