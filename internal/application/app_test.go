package application

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xenitab/go-oidc-middleware/optest"
	"golang.org/x/sync/errgroup"
)

func TestApp(t *testing.T) {
	regoTempDir := t.TempDir()
	backendPort, closeBackend := testGetTestBackend(t)
	defer closeBackend()

	testWriteRequestRego(t, regoTempDir)

	frontendPort := testGetRandomAvailablePort(t)
	managementPort := testGetRandomAvailablePort(t)

	op := optest.NewTesting(t, optest.WithTestUsers(map[string]optest.TestUser{
		"test": {
			Audience:   "test",
			Subject:    "test",
			Email:      "foo@bar.baz",
			Name:       "Foo Bar",
			GivenName:  "Foo",
			FamilyName: "Bar",
			Locale:     "en",
			ExtraAccessTokenClaims: map[string]interface{}{
				"appid": "ze-only-valid-app",
				"tid":   "ze-only-valid-tenant-id",
			},
		},
	}))
	defer op.Close(t)

	resetOsArgs := testSetOsArgs(t, []string{
		"app",
		"-v",
		"--directory", regoTempDir,
		"--backend-scheme", "http",
		"--backend-host", "localhost",
		"--backend-port", fmt.Sprintf("%d", backendPort),
		"--listen", fmt.Sprintf(":%d", frontendPort),
		"--management", fmt.Sprintf(":%d", managementPort),
		"--azure-tenant", "ze-only-valid-tenant-id",
	})
	defer resetOsArgs()

	app, ok := New()
	require.True(t, ok)
	require.NotNil(t, app)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return app.Run(gCtx)
	})

	testWaitForFrontend(t, frontendPort)

	httpClient := http.Client{}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d", frontendPort), http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", op.GetToken(t).AccessToken))

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	cancel()
	err = g.Wait()
	require.NoError(t, err)
}

func testSetOsArgs(t *testing.T, args []string) func() {
	t.Helper()

	original := os.Args
	os.Args = args
	return func() {
		os.Args = original
	}
}

func testGetRandomAvailablePort(t *testing.T) int {
	t.Helper()

	testHttpBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	addrStr := testHttpBackend.Listener.Addr().String()
	portStr := strings.Split(addrStr, ":")
	require.Len(t, portStr, 2)
	port, err := strconv.Atoi(portStr[1])
	require.NoError(t, err)

	testHttpBackend.Close()

	return port
}

func testGetTestBackend(t *testing.T) (int, func()) {
	t.Helper()

	testHttpBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	addrStr := testHttpBackend.Listener.Addr().String()
	portStr := strings.Split(addrStr, ":")
	require.Len(t, portStr, 2)
	port, err := strconv.Atoi(portStr[1])
	require.NoError(t, err)

	return port, testHttpBackend.Close
}

func testWaitForFrontend(t *testing.T, port int) {
	t.Helper()

	for range 20 {
		time.Sleep(10 * time.Millisecond)

		resp, err := http.Get(fmt.Sprintf("http://localhost:%d", port))
		if err == nil {
			resp.Body.Close()
			return
		}
	}

	require.Fail(t, "failed to connect to frontend")
}

func testWriteRequestRego(t *testing.T, dir string) {
	t.Helper()

	rego := `package request.rego

default allow := false

allow if {
	valid_apps := {
		"ze-only-valid-app",
	}
	input.request.id in valid_apps
}`

	err := os.WriteFile(fmt.Sprintf("%s/request.rego", dir), []byte(rego), 0644)
	require.NoError(t, err)
}
