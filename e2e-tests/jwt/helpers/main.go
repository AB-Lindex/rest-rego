// Package main provides a self-contained helper server for the JWT memory-leak
// investigation e2e scenario. It acts as:
//   - The backend that rest-rego proxies to
//   - An OIDC well-known endpoint
//   - A JWKS endpoint (serving the public key used for token signing)
//   - A /token endpoint that issues short-lived signed JWTs (for k6 VU init)
//
// All keys are generated in-memory at startup and live only for the duration
// of the process; they are never written to disk.
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	e2eshared "github.com/AB-Lindex/rest-rego/e2e-tests/shared"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

const (
	backendPort  = 18304
	helperPort   = 18305
	helperAddr   = "http://127.0.0.1:18305"
	testAudience = "jwt-e2e-test"
	keyID        = "jwt-e2e-key-1"
)

func main() {
	// --- Generate RSA key pair ---
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate RSA key: %v\n", err)
		os.Exit(1)
	}

	privateJWK, err := jwk.FromRaw(privateKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create private JWK: %v\n", err)
		os.Exit(1)
	}
	_ = privateJWK.Set(jwk.KeyIDKey, keyID)
	_ = privateJWK.Set(jwk.AlgorithmKey, jwa.RS256)

	publicJWK, err := jwk.FromRaw(&privateKey.PublicKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create public JWK: %v\n", err)
		os.Exit(1)
	}
	_ = publicJWK.Set(jwk.KeyIDKey, keyID)
	_ = publicJWK.Set(jwk.AlgorithmKey, jwa.RS256)

	publicSet := jwk.NewSet()
	_ = publicSet.AddKey(publicJWK)

	jwksJSON, err := json.Marshal(publicSet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal JWKS: %v\n", err)
		os.Exit(1)
	}

	// --- Backend server (proxied by rest-rego) ---
	backendServer, err := e2eshared.NewBackendServer(backendPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start backend server: %v\n", err)
		os.Exit(1)
	}
	backendServer.Mount("jwt-allow", 200, "ok")

	// --- Helper HTTP server (OIDC + token issuer) ---
	mux := http.NewServeMux()

	// OIDC well-known endpoint – rest-rego fetches this at startup
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
  "issuer": "%s",
  "jwks_uri": "%s/jwks",
  "id_token_signing_alg_values_supported": ["RS256"]
}`, helperAddr, helperAddr)
	})

	// JWKS endpoint
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksJSON)
	})

	// Token endpoint – k6 VUs call this once during setup to get a bearer token
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		tok := jwt.New()
		_ = tok.Set(jwt.SubjectKey, "e2e-test-user")
		_ = tok.Set(jwt.AudienceKey, testAudience)
		_ = tok.Set(jwt.IssuerKey, helperAddr)
		_ = tok.Set(jwt.IssuedAtKey, time.Now().Unix())
		// Long-lived so the token stays valid throughout the k6 run
		_ = tok.Set(jwt.ExpirationKey, time.Now().Add(24*time.Hour).Unix())
		_ = tok.Set("email", "e2e@example.com")
		_ = tok.Set("roles", []string{"reader"})

		signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256, privateJWK))
		if err != nil {
			http.Error(w, "token signing failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Write(signed)
	})

	helperServer := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", helperPort),
		Handler: mux,
	}

	go func() {
		if err := helperServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "helper server error: %v\n", err)
		}
	}()

	fmt.Printf("BACKEND:  http://127.0.0.1:%d\n", backendPort)
	fmt.Printf("HELPER:   %s\n", helperAddr)
	fmt.Printf("WELL-KNOWN: %s/.well-known/openid-configuration\n", helperAddr)
	fmt.Printf("JWKS:     %s/jwks\n", helperAddr)
	fmt.Printf("TOKEN:    %s/token\n", helperAddr)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = helperServer.Shutdown(shutCtx)
	backendServer.Close()
}
