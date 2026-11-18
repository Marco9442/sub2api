package ip

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// PeerTunnel is the direct peer that carries public traffic.
	PeerTunnel = "tunnel"
	// PeerMesh is the direct peer that carries the admin door.
	PeerMesh = "mesh"
	// PeerUnknown is any direct peer that is not a configured entrance.
	PeerUnknown = "unknown"

	// SourceClient means ClientIP is a verified client address.
	SourceClient = "client"
	// SourceProxy means the direct peer is a known entrance and no client address was verified.
	SourceProxy = "proxy"
	// SourceUnknown means the direct peer is not a known entrance.
	SourceUnknown = "unknown"

	defaultEgressMaxAge = 24 * time.Hour
	peerCacheTTL        = 30 * time.Second
	peerCacheKeep       = 5 * time.Minute
	entranceIdentityKey = "sub2api.entrance_identity"
)

// Identity is the single entrance decision for one request.
type Identity struct {
	ClientIP string
	Source   string
	PeerName string
}

const paymentRecordMaxLen = 50

// BindingKey is the stable session fingerprint component.
// A verified client uses that address. A known entrance without a client uses
// the peer name, so a container address change does not rotate the session.
func (id Identity) BindingKey() string {
	if id.ClientIP != "" {
		return id.ClientIP
	}
	return id.proxyMarker()
}

// PaymentRecord is the value stored on a new order.
// It is a verified address, a proxy marker, or "unknown".
// The order column accepts at most 50 characters.
func (id Identity) PaymentRecord() string {
	record := id.ClientIP
	if record == "" {
		record = id.proxyMarker()
	}
	if record == "" && id.Source == SourceUnknown {
		record = "unknown"
	}
	if len(record) > paymentRecordMaxLen {
		return record[:paymentRecordMaxLen]
	}
	return record
}

func (id Identity) proxyMarker() string {
	if id.Source == SourceProxy && id.PeerName != "" {
		return "proxy:" + id.PeerName
	}
	return ""
}

// ChainInput is the header and list state used to decide a client address.
type ChainInput struct {
	Peer         string
	PeerName     string
	ConnectingIP string
	ForwardedFor string
	List         *EgressList
}

// ResolveChain decides the verified client address for one entrance.
func ResolveChain(in ChainInput) Identity {
	switch in.Peer {
	case PeerMesh:
		return Identity{Source: SourceProxy, PeerName: in.PeerName}
	case PeerTunnel:
		return resolveTunnel(in)
	default:
		return Identity{Source: SourceUnknown}
	}
}

func resolveTunnel(in ChainInput) Identity {
	proxy := Identity{Source: SourceProxy, PeerName: in.PeerName}
	connecting, ok := canonicalAddr(in.ConnectingIP)
	if !ok || !isPublicAddr(connecting) {
		return proxy
	}
	if possibleEgress(connecting, in.List) {
		return resolveEgressChain(proxy, connecting, in.ForwardedFor)
	}
	return Identity{ClientIP: connecting.String(), Source: SourceClient, PeerName: in.PeerName}
}

func possibleEgress(connecting netip.Addr, list *EgressList) bool {
	if connecting.Is4() {
		return list.TrustIPv4() && list.contains(connecting)
	}
	if list.HasIPv6Prefixes() {
		return list.contains(connecting)
	}
	return isPublicAddr(connecting)
}

func resolveEgressChain(proxy Identity, connecting netip.Addr, forwardedFor string) Identity {
	parts := rawHops(forwardedFor)
	if len(parts) == 0 {
		return connectingClient(proxy, connecting)
	}
	last, ok := canonicalAddr(parts[len(parts)-1])
	if !ok || last != connecting {
		return proxy
	}
	if len(parts) == 1 {
		return connectingClient(proxy, connecting)
	}
	left, ok := canonicalAddr(parts[len(parts)-2])
	if !ok {
		return proxy
	}
	if left == connecting {
		return connectingClient(proxy, connecting)
	}
	if !isPublicAddr(left) {
		return proxy
	}
	return Identity{ClientIP: left.String(), Source: SourceClient, PeerName: proxy.PeerName}
}

func connectingClient(proxy Identity, connecting netip.Addr) Identity {
	return Identity{ClientIP: connecting.String(), Source: SourceClient, PeerName: proxy.PeerName}
}

func rawHops(forwardedFor string) []string {
	if strings.TrimSpace(forwardedFor) == "" {
		return nil
	}
	return strings.Split(forwardedFor, ",")
}

func canonicalAddr(raw string) (netip.Addr, bool) {
	raw = strings.TrimSpace(raw)
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	raw = strings.Trim(raw, "[]")
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap().WithZone(""), true
}

var (
	cgnatNet = mustPrefix("100.64.0.0/10")
	v6DocNet = mustPrefix("2001:db8::/32")
)

func mustPrefix(cidr string) netip.Prefix {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		panic(err)
	}
	return prefix.Masked()
}

func isPublicAddr(addr netip.Addr) bool {
	if !addr.IsValid() || addr.IsUnspecified() || addr.IsLoopback() || addr.IsMulticast() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
		return false
	}
	if addr.Is4() {
		return !cgnatNet.Contains(addr)
	}
	return addr.IsGlobalUnicast() && !v6DocNet.Contains(addr)
}

// EgressList is a parsed origin-address file.
type EgressList struct {
	FetchedAt time.Time
	MaxAge    time.Duration
	Now       time.Time
	v4        []netip.Prefix
	v6        []netip.Prefix
}

// NewEgressList parses CIDR strings into a list snapshot.
func NewEgressList(ipv4, ipv6 []string, fetchedAt time.Time, maxAge time.Duration, now time.Time) (*EgressList, error) {
	v4, err := parsePrefixes(ipv4)
	if err != nil {
		return nil, err
	}
	v6, err := parsePrefixes(ipv6)
	if err != nil {
		return nil, err
	}
	if maxAge <= 0 {
		maxAge = defaultEgressMaxAge
	}
	return &EgressList{FetchedAt: fetchedAt, MaxAge: maxAge, Now: now, v4: v4, v6: v6}, nil
}

func parsePrefixes(raw []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(raw))
	for _, item := range raw {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(item))
		if err != nil {
			return nil, fmt.Errorf("invalid egress prefix %q", item)
		}
		out = append(out, prefix.Masked())
	}
	return out, nil
}

func (l *EgressList) fresh() bool {
	if l == nil || l.FetchedAt.IsZero() {
		return false
	}
	now := l.Now
	if now.IsZero() {
		now = time.Now()
	}
	maxAge := l.MaxAge
	if maxAge <= 0 {
		maxAge = defaultEgressMaxAge
	}
	return !now.After(l.FetchedAt.Add(maxAge))
}

// withNow returns a request-local copy. Prefix slices stay shared and must not be mutated.
func (l *EgressList) withNow(now time.Time) *EgressList {
	if l == nil {
		return nil
	}
	next := *l
	next.Now = now
	return &next
}

// TrustIPv4 reports whether IPv4 origin ranges may be used.
func (l *EgressList) TrustIPv4() bool {
	return l.fresh() && len(l.v4) > 0
}

// HasIPv6Prefixes reports whether the file replaces the public-IPv6 rule.
func (l *EgressList) HasIPv6Prefixes() bool {
	return l.fresh() && len(l.v6) > 0
}

func (l *EgressList) contains(addr netip.Addr) bool {
	if l == nil {
		return false
	}
	prefixes := l.v4
	if addr.Is6() {
		prefixes = l.v6
	}
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

type egressFileJSON struct {
	FetchedAt time.Time `json:"fetched_at"`
	IPv4      []string  `json:"ipv4"`
	IPv6      []string  `json:"ipv6"`
}

// ParseEgressFile reads a normalized origin list.
// A missing fetched_at is rejected so an unofficial file cannot widen trust.
func ParseEgressFile(path string, now time.Time, maxAge time.Duration) (*EgressList, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw egressFileJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw.FetchedAt.IsZero() {
		return nil, errors.New("egress list missing fetched_at")
	}
	return NewEgressList(raw.IPv4, raw.IPv6, raw.FetchedAt.UTC(), maxAge, now)
}

// Entrance classifies the direct TCP peer and loads the origin list.
type Entrance struct {
	TunnelPeer string
	MeshPeer   string
	EgressFile string
	MaxAge     time.Duration
	Resolve    func(name string) ([]string, error)
	Now        func() time.Time
	List       *EgressList

	mu      sync.Mutex
	peers   map[string]peerCache
	flights map[string]*peerCall
	file    fileCache
}

type peerCache struct {
	at         time.Time
	resolvedAt time.Time
	ips        []netip.Addr
	ok         bool
	mismatch   bool
}

func (c peerCache) trusted(now time.Time) bool {
	return c.ok && !c.resolvedAt.IsZero() && now.Sub(c.resolvedAt) < peerCacheKeep
}

type peerCall struct {
	done chan struct{}
	ips  []netip.Addr
	err  error
}

type fileCache struct {
	mod  time.Time
	list *EgressList
	ok   bool
}

func (e *Entrance) enabled() bool {
	return e != nil && (strings.TrimSpace(e.TunnelPeer) != "" || strings.TrimSpace(e.MeshPeer) != "")
}

func (e *Entrance) now() time.Time {
	if e != nil && e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e *Entrance) maxAge() time.Duration {
	if e != nil && e.MaxAge > 0 {
		return e.MaxAge
	}
	return defaultEgressMaxAge
}

func (e *Entrance) identify(remoteAddr, connecting, forwardedFor string) Identity {
	kind, name := e.classify(remoteAddr)
	return ResolveChain(ChainInput{
		Peer:         kind,
		PeerName:     name,
		ConnectingIP: connecting,
		ForwardedFor: forwardedFor,
		List:         e.snapshot(),
	})
}

func (e *Entrance) classify(remoteAddr string) (string, string) {
	addr, ok := canonicalAddr(remoteAddr)
	if !ok {
		return PeerUnknown, ""
	}
	if kind, name, ok := e.cachedPeer(addr); ok {
		return kind, name
	}
	if e.match(e.MeshPeer, addr) {
		return PeerMesh, e.MeshPeer
	}
	if e.match(e.TunnelPeer, addr) {
		return PeerTunnel, e.TunnelPeer
	}
	return PeerUnknown, ""
}

func (e *Entrance) cachedPeer(addr netip.Addr) (string, string, bool) {
	if e.cacheHas(e.MeshPeer, addr) {
		return PeerMesh, strings.TrimSpace(e.MeshPeer), true
	}
	if e.cacheHas(e.TunnelPeer, addr) {
		return PeerTunnel, strings.TrimSpace(e.TunnelPeer), true
	}
	return "", "", false
}

func (e *Entrance) cacheHas(name string, addr netip.Addr) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	now := e.now()
	e.mu.Lock()
	cached, hit := e.peers[name]
	e.mu.Unlock()
	return hit && cached.trusted(now) && containsAddr(cached.ips, addr)
}

func (e *Entrance) match(name string, addr netip.Addr) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	ips, fromCache, err := e.lookup(name)
	if err == nil && containsAddr(ips, addr) {
		return true
	}
	if err != nil || !fromCache {
		return false
	}
	if !e.allowMismatchRefresh(name) {
		ips, _, err = e.lookup(name)
		return err == nil && containsAddr(ips, addr)
	}
	ips, err = e.resolveInto(name, true)
	return err == nil && containsAddr(ips, addr)
}

func containsAddr(ips []netip.Addr, addr netip.Addr) bool {
	for _, ip := range ips {
		if ip == addr {
			return true
		}
	}
	return false
}

func (e *Entrance) lookup(name string) ([]netip.Addr, bool, error) {
	now := e.now()
	e.mu.Lock()
	cached, hit := e.peers[name]
	e.mu.Unlock()
	if hit && now.Sub(cached.at) < peerCacheTTL {
		if cached.trusted(now) {
			return cached.ips, true, nil
		}
		return nil, true, errors.New("entrance peer lookup failed recently")
	}
	ips, err := e.resolveInto(name, false)
	return ips, false, err
}

func (e *Entrance) allowMismatchRefresh(name string) bool {
	now := e.now()
	e.mu.Lock()
	defer e.mu.Unlock()
	cached, hit := e.peers[name]
	if !hit || !cached.ok {
		return false
	}
	if now.Sub(cached.at) >= peerCacheTTL {
		return true
	}
	return !cached.mismatch
}

func (e *Entrance) resolveInto(name string, mismatch bool) ([]netip.Addr, error) {
	call, leader := e.startFlight(name)
	if !leader {
		<-call.done
		return call.ips, call.err
	}
	raw, err := e.resolve(name)
	ips := parseResolved(raw)
	now := e.now()
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.peers == nil {
		e.peers = map[string]peerCache{}
	}
	cached, hit := e.peers[name]
	if err != nil || len(ips) == 0 {
		if hit && cached.trusted(now) {
			e.peers[name] = peerCache{
				at:         now,
				resolvedAt: cached.resolvedAt,
				ips:        cached.ips,
				ok:         true,
				mismatch:   true,
			}
			call.ips = cached.ips
		} else {
			e.peers[name] = peerCache{at: now, ok: false, mismatch: mismatch}
			if err == nil {
				err = errors.New("entrance peer resolved no addresses")
			}
			call.err = err
		}
	} else {
		e.peers[name] = peerCache{at: now, resolvedAt: now, ips: ips, ok: true, mismatch: mismatch}
		call.ips = ips
	}
	delete(e.flights, name)
	close(call.done)
	return call.ips, call.err
}

func parseResolved(raw []string) []netip.Addr {
	ips := make([]netip.Addr, 0, len(raw))
	for _, item := range raw {
		addr, ok := canonicalAddr(item)
		if ok {
			ips = append(ips, addr)
		}
	}
	return ips
}

func (e *Entrance) startFlight(name string) (*peerCall, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.flights == nil {
		e.flights = map[string]*peerCall{}
	}
	if call, ok := e.flights[name]; ok {
		return call, false
	}
	call := &peerCall{done: make(chan struct{})}
	e.flights[name] = call
	return call, true
}

func (e *Entrance) resolve(name string) ([]string, error) {
	if e != nil && e.Resolve != nil {
		return e.Resolve(name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", name)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr.Unmap().WithZone("").String())
	}
	return out, nil
}

func (e *Entrance) snapshot() *EgressList {
	if e == nil {
		return nil
	}
	if strings.TrimSpace(e.EgressFile) == "" {
		return e.List
	}
	now := e.now()
	info, err := os.Stat(e.EgressFile)
	e.mu.Lock()
	defer e.mu.Unlock()
	if err != nil {
		return nil
	}
	if e.file.ok && e.file.list != nil && e.file.mod.Equal(info.ModTime()) {
		return e.file.list.withNow(now)
	}
	parsed, parseErr := ParseEgressFile(e.EgressFile, now, e.maxAge())
	if parseErr != nil {
		if e.file.ok && e.file.list != nil {
			copy := e.file.list.withNow(now)
			if copy.fresh() {
				return copy
			}
		}
		return nil
	}
	e.file = fileCache{mod: info.ModTime(), list: parsed, ok: true}
	return parsed.withNow(now)
}

var entranceState atomic.Pointer[Entrance]

// SetEntrance installs the process-wide entrance resolver.
// A nil value restores the legacy client-IP path.
func SetEntrance(e *Entrance) {
	entranceState.Store(e)
}

// ConfigureEntranceFromEnv enables entrance resolution when a peer name is set.
// Unset peer names leave the legacy path unchanged.
func ConfigureEntranceFromEnv() {
	tunnel := strings.TrimSpace(os.Getenv("SECURITY_ENTRANCE_TUNNEL_PEER"))
	mesh := strings.TrimSpace(os.Getenv("SECURITY_ENTRANCE_MESH_PEER"))
	if tunnel == "" && mesh == "" {
		SetEntrance(nil)
		return
	}
	maxAge := defaultEgressMaxAge
	if raw := strings.TrimSpace(os.Getenv("SECURITY_ENTRANCE_EGRESS_MAX_AGE")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			maxAge = parsed
		}
	}
	SetEntrance(&Entrance{
		TunnelPeer: tunnel,
		MeshPeer:   mesh,
		EgressFile: strings.TrimSpace(os.Getenv("SECURITY_ENTRANCE_EGRESS_FILE")),
		MaxAge:     maxAge,
	})
}

// CurrentIdentity returns the entrance decision for this request.
// The second result is false when entrance resolution is not configured.
func CurrentIdentity(c *gin.Context) (Identity, bool) {
	e := entranceState.Load()
	if !e.enabled() {
		return Identity{}, false
	}
	if c == nil || c.Request == nil {
		return Identity{Source: SourceUnknown}, true
	}
	if value, ok := c.Get(entranceIdentityKey); ok {
		if id, ok := value.(Identity); ok {
			return id, true
		}
	}
	id := e.identify(c.Request.RemoteAddr, c.GetHeader("CF-Connecting-IP"), c.GetHeader("X-Forwarded-For"))
	c.Set(entranceIdentityKey, id)
	return id, true
}

// PaymentAddresses returns the order record and the provider payload address.
// The provider value is a verified client address, or empty when none exists.
func PaymentAddresses(c *gin.Context) (string, string) {
	if id, ok := CurrentIdentity(c); ok {
		if id.ClientIP != "" {
			return id.ClientIP, id.ClientIP
		}
		return id.PaymentRecord(), ""
	}
	raw := ""
	if c != nil {
		raw = normalizeIP(c.ClientIP())
	}
	return raw, raw
}
