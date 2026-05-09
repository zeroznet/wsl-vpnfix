// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Haiku 4.5)

package healthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProbeHTTP_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := ProbeHTTP(context.Background(), srv.URL, 2*time.Second, nil)
	assert.True(t, res.OK)
	assert.Equal(t, 200, res.StatusCode)
}

func TestProbeHTTP_TLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// httptest's TLS server uses a self-signed cert. The test server exposes
	// a Client() with that cert already trusted; we use it via the internal
	// probe entrypoint. Production callers pass nil tlsConf and rely on
	// system root CAs.
	res := probeHTTPWithClient(context.Background(), srv.URL, 2*time.Second, srv.Client())
	assert.True(t, res.OK)
	assert.Equal(t, 200, res.StatusCode)
}

func TestProbeHTTP_Timeout(t *testing.T) {
	res := ProbeHTTP(context.Background(), "http://127.0.0.1:1", 200*time.Millisecond, nil)
	assert.False(t, res.OK)
	assert.NotEmpty(t, res.Err)
}

func TestProbeDNS_ResolvesLocalhost(t *testing.T) {
	res := ProbeDNS(context.Background(), "localhost", "", 2*time.Second)
	assert.True(t, res.OK, "localhost must resolve via system resolver, got err: %s", res.Err)
}
