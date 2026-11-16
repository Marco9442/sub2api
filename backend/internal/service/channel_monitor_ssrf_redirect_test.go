//go:build unit

package service

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMonitorCheckRedirectRejectsPublicChainIntoLoopback(t *testing.T) {
	public := httptest.NewRequest(http.MethodPost, "https://upstream.example/v1/chat", nil)
	again := httptest.NewRequest(http.MethodPost, "https://cdn.example/v1/chat", nil)
	local := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/secret", nil)
	otherLocal := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/v1/models", nil)

	require.ErrorIs(t, monitorCheckRedirect(local, []*http.Request{public}), errMonitorRedirectToLoopback)
	require.ErrorIs(t, monitorCheckRedirect(local, []*http.Request{public, again}), errMonitorRedirectToLoopback)
	require.ErrorIs(t, monitorCheckRedirect(local, []*http.Request{otherLocal, public}), errMonitorRedirectToLoopback)
	require.NoError(t, monitorCheckRedirect(again, []*http.Request{public}))
	require.NoError(t, monitorCheckRedirect(otherLocal, []*http.Request{local}))
}

func TestMonitorCheckRedirectKeepsTenHopLimit(t *testing.T) {
	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = httptest.NewRequest(http.MethodGet, "https://upstream.example/v1", nil)
	}
	next := httptest.NewRequest(http.MethodGet, "https://cdn.example/v1", nil)

	err := monitorCheckRedirect(next, via)
	require.Error(t, err)
	require.NotErrorIs(t, err, errMonitorRedirectToLoopback)
	require.Contains(t, err.Error(), "stopped after 10 redirects")
}

func TestMonitorClientsShareRedirectCheck(t *testing.T) {
	want := reflect.ValueOf(monitorCheckRedirect).Pointer()
	for _, client := range []*http.Client{
		newSSRFSafeHTTPClient(time.Second),
		newCustomMonitorHTTPClient(),
	} {
		require.Equal(t, want, reflect.ValueOf(client.CheckRedirect).Pointer())
	}
}

func TestPublicRedirectDoesNotDialLoopbackException(t *testing.T) {
	var dialed []string
	client := &http.Client{
		CheckRedirect: monitorCheckRedirect,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				dialed = append(dialed, address)
				if address == "127.0.0.1:8080" {
					t.Errorf("public redirect dialed %s", address)
					return nil, errMonitorRedirectToLoopback
				}
				return (&net.Dialer{}).DialContext(ctx, network, address)
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bad":
			http.Redirect(w, r, "http://127.0.0.1:8080/secret", http.StatusTemporaryRedirect)
		case "/ok":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Redirect(w, r, "/ok", http.StatusFound)
		}
	}))
	t.Cleanup(srv.Close)

	resp, err := client.Get(srv.URL + "/start")
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	_, err = client.Post(srv.URL+"/bad", "application/json", nil)
	require.ErrorIs(t, err, errMonitorRedirectToLoopback)
	for _, address := range dialed {
		require.NotEqual(t, "127.0.0.1:8080", address)
	}
}
