// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Haiku 4.5)

// Package healthcheck runs informational connectivity probes after wsl-vpnfix
// finishes setup. Probes never gate startup; they are logged and returned.
package healthcheck

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Result of a single probe.
type Result struct {
	Name       string
	OK         bool
	StatusCode int
	Err        string
	Elapsed    time.Duration
}

// ProbeHTTP does a GET against url with the given timeout. Any URL scheme
// supported by net/http works (http://, https://). If tlsConf is non-nil it
// is used to override default TLS config — pass nil for system-roots TLS.
func ProbeHTTP(ctx context.Context, url string, timeout time.Duration, tlsConf *tls.Config) Result {
	cli := &http.Client{Timeout: timeout}
	if tlsConf != nil {
		cli.Transport = &http.Transport{TLSClientConfig: tlsConf}
	}
	return probeHTTPWithClient(ctx, url, timeout, cli)
}

func probeHTTPWithClient(ctx context.Context, url string, timeout time.Duration, cli *http.Client) Result {
	t0 := time.Now()
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(c, http.MethodGet, url, nil)
	if err != nil {
		return Result{Name: "http:" + url, OK: false, Err: err.Error(), Elapsed: time.Since(t0)}
	}
	resp, err := cli.Do(req)
	if err != nil {
		return Result{Name: "http:" + url, OK: false, Err: err.Error(), Elapsed: time.Since(t0)}
	}
	defer resp.Body.Close()
	return Result{
		Name:       "http:" + url,
		OK:         resp.StatusCode >= 200 && resp.StatusCode < 400,
		StatusCode: resp.StatusCode,
		Elapsed:    time.Since(t0),
	}
}

// ProbeDNS resolves host using server (or system default if server == "").
func ProbeDNS(ctx context.Context, host, server string, timeout time.Duration) Result {
	t0 := time.Now()
	r := &net.Resolver{}
	if server != "" {
		r.PreferGo = true
		r.Dial = func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: timeout}
			return d.DialContext(ctx, "udp", net.JoinHostPort(server, "53"))
		}
	}
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	addrs, err := r.LookupHost(c, host)
	if err != nil {
		return Result{Name: fmt.Sprintf("dns:%s@%s", host, server), OK: false, Err: err.Error(), Elapsed: time.Since(t0)}
	}
	return Result{
		Name:    fmt.Sprintf("dns:%s@%s", host, server),
		OK:      len(addrs) > 0,
		Elapsed: time.Since(t0),
	}
}
