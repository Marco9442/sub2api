//go:build unit

package ip

import (
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResolveChainMainIPv4IgnoresForgedLeftmost(t *testing.T) {
	got := ResolveChain(ChainInput{
		Peer:         PeerTunnel,
		PeerName:     "baima-edge",
		ConnectingIP: "203.0.113.10",
		ForwardedFor: "192.0.2.11, 203.0.113.10",
		List:         testList(t, []string{"198.51.100.0/24"}, nil, time.Minute),
	})
	require.Equal(t, "203.0.113.10", got.ClientIP)
	require.Equal(t, SourceClient, got.Source)
}

func TestResolveChainListedIPv4UsesImmediateLeftHop(t *testing.T) {
	got := ResolveChain(ChainInput{
		Peer:         PeerTunnel,
		PeerName:     "baima-edge",
		ConnectingIP: "198.51.100.8",
		ForwardedFor: "192.0.2.11, 203.0.113.10, 198.51.100.8",
		List:         testList(t, []string{"198.51.100.0/24"}, nil, time.Minute),
	})
	require.Equal(t, "203.0.113.10", got.ClientIP)
	require.Equal(t, SourceClient, got.Source)
}

func TestResolveChainListedIPv4WithoutPreviousHopIsClient(t *testing.T) {
	list := testList(t, []string{"198.51.100.0/24"}, nil, time.Minute)
	for _, xff := range []string{"", "198.51.100.8", " 198.51.100.8 "} {
		got := ResolveChain(ChainInput{
			Peer:         PeerTunnel,
			PeerName:     "baima-edge",
			ConnectingIP: "198.51.100.8",
			ForwardedFor: xff,
			List:         list,
		})
		require.Equal(t, "198.51.100.8", got.ClientIP, xff)
		require.Equal(t, SourceClient, got.Source)
	}
}

func TestResolveChainPublicIPv6WithLeftHop(t *testing.T) {
	got := ResolveChain(ChainInput{
		Peer:         PeerTunnel,
		PeerName:     "baima-edge",
		ConnectingIP: "2600:3c18::1",
		ForwardedFor: "192.0.2.11, 203.0.113.10, 2600:3c18::1",
		List:         testList(t, []string{"198.51.100.0/24"}, nil, time.Minute),
	})
	require.Equal(t, "203.0.113.10", got.ClientIP)
}

func TestResolveChainPublicIPv6WithoutPreviousHopIsClient(t *testing.T) {
	list := testList(t, []string{"198.51.100.0/24"}, nil, time.Minute)
	for _, xff := range []string{"", "2600:3c18::1"} {
		got := ResolveChain(ChainInput{
			Peer:         PeerTunnel,
			PeerName:     "baima-edge",
			ConnectingIP: "2600:3c18::1",
			ForwardedFor: xff,
			List:         list,
		})
		require.Equal(t, "2600:3c18::1", got.ClientIP, xff)
		require.Equal(t, SourceClient, got.Source)
		require.NotEmpty(t, got.ClientIP)
	}
}

func TestResolveChainInvalidRawHopIsBroken(t *testing.T) {
	list := testList(t, []string{"198.51.100.0/24"}, nil, time.Minute)
	broken := []ChainInput{
		{
			Peer: PeerTunnel, PeerName: "baima-edge",
			ConnectingIP: "198.51.100.8",
			ForwardedFor: "203.0.113.10, not-an-ip, 198.51.100.8",
			List:         list,
		},
		{
			Peer: PeerTunnel, PeerName: "baima-edge",
			ConnectingIP: "198.51.100.8",
			ForwardedFor: "203.0.113.10, 198.51.100.8, not-an-ip",
			List:         list,
		},
		{
			Peer: PeerTunnel, PeerName: "baima-edge",
			ConnectingIP: "2600:3c18::1",
			ForwardedFor: "not-an-ip, 2600:3c18::1",
			List:         list,
		},
	}
	for _, in := range broken {
		got := ResolveChain(in)
		require.Empty(t, got.ClientIP, in.ForwardedFor)
		require.Equal(t, SourceProxy, got.Source, in.ForwardedFor)
	}

	for _, xff := range []string{"", "2600:3c18::1", "2600:3c18::1, 2600:3c18::1"} {
		got := ResolveChain(ChainInput{
			Peer: PeerTunnel, PeerName: "baima-edge",
			ConnectingIP: "2600:3c18::1",
			ForwardedFor: xff,
			List:         list,
		})
		require.Equal(t, "2600:3c18::1", got.ClientIP, xff)
		require.Equal(t, SourceClient, got.Source, xff)
	}
}

func TestResolveChainBrokenChainHasNoClient(t *testing.T) {
	list := testList(t, []string{"198.51.100.0/24"}, nil, time.Minute)
	cases := []ChainInput{
		{
			Peer: PeerTunnel, PeerName: "baima-edge",
			ConnectingIP: "2600:3c18::1",
			ForwardedFor: "203.0.113.10, 198.51.100.8",
			List:         list,
		},
		{
			Peer: PeerTunnel, PeerName: "baima-edge",
			ConnectingIP: "198.51.100.8",
			ForwardedFor: "10.1.2.3, 198.51.100.8",
			List:         list,
		},
		{
			Peer: PeerTunnel, PeerName: "baima-edge",
			ConnectingIP: "2600:3c18::1",
			ForwardedFor: "fd00::1, 2600:3c18::1",
			List:         list,
		},
	}
	for _, in := range cases {
		got := ResolveChain(in)
		require.Empty(t, got.ClientIP)
		require.Equal(t, SourceProxy, got.Source)
		require.Equal(t, "proxy:baima-edge", got.BindingKey())
	}
}

func TestResolveChainIPv4MappedUsesEmbeddedIPv4(t *testing.T) {
	got := ResolveChain(ChainInput{
		Peer:         PeerTunnel,
		PeerName:     "baima-edge",
		ConnectingIP: "::ffff:198.51.100.8",
		ForwardedFor: "::ffff:203.0.113.10, ::ffff:198.51.100.8",
		List:         testList(t, []string{"198.51.100.0/24"}, nil, time.Minute),
	})
	require.Equal(t, "203.0.113.10", got.ClientIP)
}

func TestResolveChainNonPublicIPv6IsNotEgress(t *testing.T) {
	list := testList(t, nil, nil, time.Minute)
	for _, connecting := range []string{"2001:db8::1", "fd00::1", "fe80::1", "ff02::1", "::1"} {
		got := ResolveChain(ChainInput{
			Peer:         PeerTunnel,
			PeerName:     "baima-edge",
			ConnectingIP: connecting,
			ForwardedFor: "203.0.113.10, " + connecting,
			List:         list,
		})
		require.Empty(t, got.ClientIP, connecting)
		require.Equal(t, SourceProxy, got.Source)
	}
}

func TestResolveChainMeshAndUnknownIgnoreForwardedHeaders(t *testing.T) {
	list := testList(t, []string{"198.51.100.0/24"}, nil, time.Minute)
	mesh := ResolveChain(ChainInput{
		Peer: PeerMesh, PeerName: "baima-mesh",
		ConnectingIP: "203.0.113.10",
		ForwardedFor: "203.0.113.10",
		List:         list,
	})
	require.Empty(t, mesh.ClientIP)
	require.Equal(t, SourceProxy, mesh.Source)
	require.Equal(t, "proxy:baima-mesh", mesh.BindingKey())
	require.Equal(t, "proxy:baima-mesh", mesh.PaymentRecord())

	unknown := ResolveChain(ChainInput{
		Peer:         PeerUnknown,
		ConnectingIP: "203.0.113.10",
		ForwardedFor: "203.0.113.9, 203.0.113.10",
		List:         list,
	})
	require.Empty(t, unknown.ClientIP)
	require.Equal(t, SourceUnknown, unknown.Source)
	require.Empty(t, unknown.BindingKey())
	require.Equal(t, "unknown", unknown.PaymentRecord())
}

func TestResolveChainMissingOrStaleListDoesNotTrustRanges(t *testing.T) {
	stale := testList(t, []string{"198.51.100.0/24"}, nil, time.Minute)
	stale.Now = stale.FetchedAt.Add(2 * time.Minute)
	got := ResolveChain(ChainInput{
		Peer: PeerTunnel, PeerName: "baima-edge",
		ConnectingIP: "198.51.100.8",
		ForwardedFor: "203.0.113.10, 198.51.100.8",
		List:         stale,
	})
	require.Equal(t, "198.51.100.8", got.ClientIP)

	missing := ResolveChain(ChainInput{
		Peer: PeerTunnel, PeerName: "baima-edge",
		ConnectingIP: "198.51.100.8",
		ForwardedFor: "203.0.113.10, 198.51.100.8",
	})
	require.Equal(t, "198.51.100.8", missing.ClientIP)

	// Public IPv6 still uses the left hop when the IPv4 list is missing.
	v6 := ResolveChain(ChainInput{
		Peer: PeerTunnel, PeerName: "baima-edge",
		ConnectingIP: "2600:3c18::1",
		ForwardedFor: "203.0.113.10, 2600:3c18::1",
	})
	require.Equal(t, "203.0.113.10", v6.ClientIP)
}

func TestResolveChainIPv6PrefixesReplacePublicIPv6Rule(t *testing.T) {
	list := testList(t, []string{"198.51.100.0/24"}, []string{"2600:3c18::/32"}, time.Minute)
	inList := ResolveChain(ChainInput{
		Peer: PeerTunnel, PeerName: "baima-edge",
		ConnectingIP: "2600:3c18::1",
		ForwardedFor: "203.0.113.10, 2600:3c18::1",
		List:         list,
	})
	require.Equal(t, "203.0.113.10", inList.ClientIP)

	outside := ResolveChain(ChainInput{
		Peer: PeerTunnel, PeerName: "baima-edge",
		ConnectingIP: "2001:4860:4860::8888",
		ForwardedFor: "203.0.113.10, 2001:4860:4860::8888",
		List:         list,
	})
	require.Equal(t, "2001:4860:4860::8888", outside.ClientIP)
}

func TestParseEgressFileAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "list.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"fetched_at": "2026-11-18T00:00:00Z",
		"ipv4": ["198.51.100.0/24"],
		"ipv6": []
	}`), 0o644))
	now := time.Date(2026, 11, 18, 1, 0, 0, 0, time.UTC)
	list, err := ParseEgressFile(path, now, 24*time.Hour)
	require.NoError(t, err)
	require.True(t, list.TrustIPv4())
	require.False(t, list.HasIPv6Prefixes())

	_, err = ParseEgressFile(filepath.Join(dir, "missing.json"), now, time.Hour)
	require.Error(t, err)

	require.NoError(t, os.WriteFile(path, []byte(`{"ipv4":["198.51.100.0/24"]}`), 0o644))
	_, err = ParseEgressFile(path, now, time.Hour)
	require.Error(t, err)
}

func TestGetClientIPUnchangedWhenEntranceDisabled(t *testing.T) {
	SetEntrance(nil)
	t.Cleanup(func() { SetEntrance(nil) })
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.GET("/t", func(c *gin.Context) {
		c.String(200, GetClientIP(c))
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/t", nil)
	req.RemoteAddr = "192.168.32.1:12345"
	req.Header.Set("X-Forwarded-For", "10.0.0.2, 203.0.113.42")
	req.Header.Set("X-Real-IP", "192.168.32.1")
	r.ServeHTTP(w, req)
	require.Equal(t, "203.0.113.42", w.Body.String())
}

func TestEntranceRequestUsesPeerAndIgnoresForge(t *testing.T) {
	SetEntrance(&Entrance{
		TunnelPeer: "baima-edge",
		MeshPeer:   "baima-mesh",
		Resolve: func(name string) ([]string, error) {
			switch name {
			case "baima-edge":
				return []string{"172.18.0.5"}, nil
			case "baima-mesh":
				return []string{"172.18.0.3"}, nil
			default:
				return nil, os.ErrNotExist
			}
		},
		List: testList(t, []string{"198.51.100.0/24"}, nil, time.Hour),
	})
	t.Cleanup(func() { SetEntrance(nil) })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.GET("/t", func(c *gin.Context) {
		id, ok := CurrentIdentity(c)
		require.True(t, ok)
		c.String(200, GetClientIP(c)+"|"+GetSecurityClientIP(c, true)+"|"+id.BindingKey())
	})

	tunnel := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/t", nil)
	req.RemoteAddr = "172.18.0.5:43210"
	req.Header.Set("CF-Connecting-IP", "203.0.113.10")
	req.Header.Set("X-Forwarded-For", "192.0.2.11, 203.0.113.10")
	req.Header.Set("X-Real-IP", "192.0.2.99")
	r.ServeHTTP(tunnel, req)
	require.Equal(t, "203.0.113.10|203.0.113.10|203.0.113.10", tunnel.Body.String())

	mesh := httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/t", nil)
	req.RemoteAddr = "172.18.0.3:43210"
	req.Header.Set("CF-Connecting-IP", "203.0.113.10")
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	req.Header.Set("X-Real-IP", "203.0.113.10")
	r.ServeHTTP(mesh, req)
	require.Equal(t, "||proxy:baima-mesh", mesh.Body.String())

	unknown := httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/t", nil)
	req.RemoteAddr = "172.18.0.9:43210"
	req.Header.Set("CF-Connecting-IP", "203.0.113.10")
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	r.ServeHTTP(unknown, req)
	require.Equal(t, "||", unknown.Body.String())
}

func TestPaymentAddressesUsesVerifiedClientOnly(t *testing.T) {
	SetEntrance(&Entrance{
		TunnelPeer: "baima-edge",
		MeshPeer:   "baima-mesh",
		Resolve: func(name string) ([]string, error) {
			if name == "baima-edge" {
				return []string{"172.18.0.5"}, nil
			}
			return []string{"172.18.0.3"}, nil
		},
		List: testList(t, []string{"198.51.100.0/24"}, nil, time.Hour),
	})
	t.Cleanup(func() { SetEntrance(nil) })

	gin.SetMode(gin.TestMode)
	tunnel, _ := gin.CreateTestContext(httptest.NewRecorder())
	tunnel.Request = httptest.NewRequest("POST", "/api/v1/payment/orders", nil)
	tunnel.Request.RemoteAddr = "172.18.0.5:1"
	tunnel.Request.Header.Set("CF-Connecting-IP", "198.51.100.8")
	tunnel.Request.Header.Set("X-Forwarded-For", "203.0.113.10, 198.51.100.8")
	record, provider := PaymentAddresses(tunnel)
	require.Equal(t, "203.0.113.10", record)
	require.Equal(t, "203.0.113.10", provider)

	mesh, _ := gin.CreateTestContext(httptest.NewRecorder())
	mesh.Request = httptest.NewRequest("POST", "/api/v1/payment/orders", nil)
	mesh.Request.RemoteAddr = "172.18.0.3:1"
	mesh.Request.Header.Set("CF-Connecting-IP", "203.0.113.10")
	record, provider = PaymentAddresses(mesh)
	require.Equal(t, "proxy:baima-mesh", record)
	require.Empty(t, provider)
}

func TestPaymentRecordFitsOrderColumn(t *testing.T) {
	id := Identity{Source: SourceProxy, PeerName: strings.Repeat("n", 80)}
	require.LessOrEqual(t, len(id.PaymentRecord()), 50)
	require.True(t, strings.HasPrefix(id.PaymentRecord(), "proxy:"))
	require.Equal(t, "proxy:"+strings.Repeat("n", 80), id.BindingKey())
}

func TestPeerLookupCachesFailureAndKeepsLastGood(t *testing.T) {
	now := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	calls := 0
	fail := false
	e := &Entrance{
		TunnelPeer: "baima-edge",
		Now:        func() time.Time { return now },
		Resolve: func(string) ([]string, error) {
			calls++
			if fail {
				return nil, errors.New("dns down")
			}
			return []string{"172.18.0.5"}, nil
		},
		List: testList(t, []string{"198.51.100.0/24"}, nil, time.Hour),
	}

	unknown := &Entrance{
		TunnelPeer: "baima-edge",
		Now:        func() time.Time { return now },
		Resolve: func(string) ([]string, error) {
			calls++
			return nil, errors.New("dns down")
		},
	}
	require.Equal(t, SourceUnknown, unknown.identify("172.18.0.5:1", "203.0.113.10", "203.0.113.10").Source)
	firstCalls := calls
	require.Equal(t, SourceUnknown, unknown.identify("172.18.0.5:1", "203.0.113.10", "203.0.113.10").Source)
	require.Equal(t, firstCalls, calls)

	got := e.identify("172.18.0.5:1", "203.0.113.10", "")
	require.Equal(t, "203.0.113.10", got.ClientIP)
	fail = true
	now = now.Add(31 * time.Second)
	got = e.identify("172.18.0.5:1", "203.0.113.10", "")
	require.Equal(t, "203.0.113.10", got.ClientIP)
	now = now.Add(peerCacheKeep)
	require.Equal(t, SourceUnknown, e.identify("172.18.0.5:1", "203.0.113.10", "").Source)
}

func TestSnapshotClockIsPrivate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "list.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"fetched_at": "2026-11-18T00:00:00Z",
		"ipv4": ["198.51.100.0/24"],
		"ipv6": []
	}`), 0o644))
	now := time.Date(2026, 11, 18, 1, 0, 0, 0, time.UTC)
	e := &Entrance{EgressFile: path, Now: func() time.Time { return now }}
	first := e.snapshot()
	require.True(t, first.TrustIPv4())
	now = now.Add(25 * time.Hour)
	second := e.snapshot()
	require.NotSame(t, first, second)
	require.True(t, first.TrustIPv4())
	require.False(t, second.TrustIPv4())
}

func TestSnapshotConcurrentFreshness(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "list.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"fetched_at": "2026-11-18T00:00:00Z",
		"ipv4": ["198.51.100.0/24"],
		"ipv6": []
	}`), 0o644))
	now := time.Date(2026, 11, 18, 1, 0, 0, 0, time.UTC)
	e := &Entrance{EgressFile: path, Now: func() time.Time { return now }}
	require.NotNil(t, e.snapshot())
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 40 {
				list := e.snapshot()
				if list == nil || !list.TrustIPv4() {
					t.Errorf("shared list clock changed freshness")
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestPeerLookupRefreshesMismatchOnce(t *testing.T) {
	now := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	calls := 0
	remote := "172.18.0.5"
	e := &Entrance{
		TunnelPeer: "baima-edge",
		Now:        func() time.Time { return now },
		Resolve: func(string) ([]string, error) {
			calls++
			return []string{remote}, nil
		},
		List: testList(t, []string{"198.51.100.0/24"}, nil, time.Hour),
	}
	require.Equal(t, "203.0.113.10", e.identify("172.18.0.5:1", "203.0.113.10", "").ClientIP)
	require.Equal(t, 1, calls)

	remote = "172.18.0.9"
	now = now.Add(5 * time.Second)
	got := e.identify("172.18.0.9:1", "203.0.113.10", "")
	require.Equal(t, "203.0.113.10", got.ClientIP)
	require.Equal(t, SourceClient, got.Source)
	require.Equal(t, 2, calls)

	now = now.Add(5 * time.Second)
	got = e.identify("172.18.0.8:1", "203.0.113.10", "")
	require.Equal(t, SourceUnknown, got.Source)
	require.Empty(t, got.ClientIP)
	require.Equal(t, 2, calls)
}

func TestPeerLookupCoalescesMismatchRefresh(t *testing.T) {
	now := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	entered := make(chan struct{}, 2)
	gate := make(chan struct{})
	e := &Entrance{
		TunnelPeer: "baima-edge",
		Now:        func() time.Time { return now },
		Resolve: func(string) ([]string, error) {
			n := calls.Add(1)
			if n == 1 {
				return []string{"172.18.0.5"}, nil
			}
			entered <- struct{}{}
			<-gate
			return []string{"172.18.0.9"}, nil
		},
		List: testList(t, []string{"198.51.100.0/24"}, nil, time.Hour),
	}
	require.Equal(t, SourceClient, e.identify("172.18.0.5:1", "203.0.113.10", "").Source)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		e.identify("172.18.0.9:1", "203.0.113.10", "")
	}()
	go func() {
		defer wg.Done()
		e.identify("172.18.0.9:1", "203.0.113.10", "")
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("mismatch did not refresh")
	}
	select {
	case <-entered:
		t.Fatal("mismatch refresh was not shared")
	case <-time.After(50 * time.Millisecond):
	}
	close(gate)
	wg.Wait()
	require.Equal(t, int32(2), calls.Load())
}

func TestPeerLookupFailureBumpsCacheClock(t *testing.T) {
	now := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	calls := 0
	fail := false
	e := &Entrance{
		TunnelPeer: "baima-edge",
		Now:        func() time.Time { return now },
		Resolve: func(string) ([]string, error) {
			calls++
			if fail {
				return nil, errors.New("dns down")
			}
			return []string{"172.18.0.5"}, nil
		},
		List: testList(t, []string{"198.51.100.0/24"}, nil, time.Hour),
	}
	require.Equal(t, "203.0.113.10", e.identify("172.18.0.5:1", "203.0.113.10", "").ClientIP)
	fail = true
	now = now.Add(31 * time.Second)
	require.Equal(t, SourceUnknown, e.identify("172.18.0.9:1", "203.0.113.10", "").Source)
	require.Equal(t, "203.0.113.10", e.identify("172.18.0.5:1", "203.0.113.10", "").ClientIP)
	afterFail := calls
	now = now.Add(5 * time.Second)
	require.Equal(t, "203.0.113.10", e.identify("172.18.0.5:1", "203.0.113.10", "").ClientIP)
	require.Equal(t, SourceUnknown, e.identify("172.18.0.8:1", "203.0.113.10", "").Source)
	require.Equal(t, afterFail, calls)
}

func TestPeerLookupFailureDoesNotRenewTrust(t *testing.T) {
	start := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	now := start
	fail := false
	e := &Entrance{
		TunnelPeer: "baima-edge",
		Now:        func() time.Time { return now },
		Resolve: func(string) ([]string, error) {
			if fail {
				return nil, errors.New("dns down")
			}
			return []string{"172.18.0.5"}, nil
		},
		List: testList(t, []string{"198.51.100.0/24"}, nil, time.Hour),
	}
	require.Equal(t, SourceClient, e.identify("172.18.0.5:1", "203.0.113.10", "").Source)
	fail = true
	now = start.Add(4*time.Minute + 40*time.Second)
	require.Equal(t, SourceClient, e.identify("172.18.0.5:1", "203.0.113.10", "").Source)
	now = start.Add(5*time.Minute + 5*time.Second)
	require.Equal(t, SourceUnknown, e.identify("172.18.0.5:1", "203.0.113.10", "").Source)
}

func TestClassifyMatchesCacheBeforeRefresh(t *testing.T) {
	now := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	meshIP := "172.18.0.3"
	tunnelIP := "172.18.0.5"
	calls := map[string]int{}
	e := &Entrance{
		TunnelPeer: "baima-edge",
		MeshPeer:   "baima-mesh",
		Now:        func() time.Time { return now },
		Resolve: func(name string) ([]string, error) {
			calls[name]++
			if name == "baima-mesh" {
				return []string{meshIP}, nil
			}
			return []string{tunnelIP}, nil
		},
		List: testList(t, []string{"198.51.100.0/24"}, nil, time.Hour),
	}

	require.Equal(t, SourceClient, e.identify(tunnelIP+":1", "203.0.113.10", "").Source)
	require.Equal(t, SourceProxy, e.identify(meshIP+":1", "203.0.113.10", "198.51.100.8, 203.0.113.10").Source)
	meshCalls := calls["baima-mesh"]

	now = now.Add(5 * time.Second)
	require.Equal(t, SourceClient, e.identify(tunnelIP+":1", "203.0.113.10", "").Source)
	require.Equal(t, meshCalls, calls["baima-mesh"])

	meshIP = "172.18.0.4"
	now = now.Add(5 * time.Second)
	got := e.identify("172.18.0.4:1", "203.0.113.10", "198.51.100.8, 203.0.113.10")
	require.Equal(t, SourceProxy, got.Source)
	require.Equal(t, "baima-mesh", got.PeerName)
	require.Empty(t, got.ClientIP)
}

func testList(t *testing.T, v4, v6 []string, maxAge time.Duration) *EgressList {
	t.Helper()
	now := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	list, err := NewEgressList(v4, v6, now, maxAge, now)
	require.NoError(t, err)
	return list
}
