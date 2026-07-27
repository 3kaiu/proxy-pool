package validator

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"

	"proxy-pool/internal/circuit"
	"proxy-pool/internal/config"
	"proxy-pool/internal/storage"
)

type Validator struct {
	cfg     config.Config
	store   *storage.Storage
	cb      *circuit.Breaker
	sem     chan struct{}
	checker *IPChecker
}

func New(cfg config.Config, store *storage.Storage) *Validator {
	return &Validator{
		cfg:     cfg,
		store:   store,
		cb:      circuit.New(cfg.CBThreshold, cfg.CBCooldown),
		sem:     make(chan struct{}, cfg.MaxValidateConcurrency),
		checker: NewIPChecker(cfg.IPCheckURLs),
	}
}

// ValidateOne validates a single proxy with protocol-awareness.
func (v *Validator) ValidateOne(ctx context.Context, proxy storage.Proxy) bool {
	if v.cb.IsOpen() {
		return false
	}

	select {
	case <-ctx.Done():
		return false
	case v.sem <- struct{}{}:
	}
	defer func() { <-v.sem }()

	// Build proxy URL with correct protocol scheme.
	// tls-client supports "http://", "socks4://", "socks5://" proxy URLs.
	proxyURL := proxy.Protocol + "://" + proxy.Addr

	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(int(v.cfg.ValidateTimeout.Seconds())),
		tls_client.WithClientProfile(profiles.Chrome_124),
		tls_client.WithProxyUrl(proxyURL),
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		log.Printf("[Validator] Failed to create tls client for %s://%s: %v", proxy.Protocol, proxy.Addr, err)
		return false
	}

	// Step 1: IP Consistency Check (MITM / Proxy Spoofing)
	checkURL, fieldName := v.checker.Pick()
	reqIP, _ := http.NewRequestWithContext(ctx, "GET", checkURL, nil)

	respIP, err := client.Do(reqIP)
	if err != nil {
		// SSL Handshake failure -> MITM suspicion or proxy doesn't support HTTPS CONNECT
		v.store.UpdateScore(proxy.Addr, proxy.Protocol, v.cfg.ScoreDownSSL, 9999)
		return false
	}
	defer respIP.Body.Close()

	if respIP.StatusCode != 200 {
		v.store.UpdateScore(proxy.Addr, proxy.Protocol, v.cfg.ScoreDown, 9999)
		return false
	}

	bodyIP, _ := io.ReadAll(respIP.Body)
	var data map[string]interface{}
	if err := json.Unmarshal(bodyIP, &data); err != nil {
		v.store.UpdateScore(proxy.Addr, proxy.Protocol, v.cfg.ScoreDownHoney, 9999) // returned HTML/garbage instead of JSON
		return false
	}

	if returnedIP, ok := data[fieldName].(string); !ok || returnedIP == "" {
		v.store.UpdateScore(proxy.Addr, proxy.Protocol, v.cfg.ScoreDownHoney, 9999)
		return false
	}

	// Step 2: Target API Reachability
	reqTarget, _ := http.NewRequestWithContext(ctx, "GET", v.cfg.TargetURL, nil)
	reqTarget.Header.Set("User-Agent", "opencode/latest/1.4.1/cli")

	start := time.Now()
	respTarget, err := client.Do(reqTarget)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		v.store.UpdateScore(proxy.Addr, proxy.Protocol, v.cfg.ScoreDown, latency)
		return false
	}
	defer respTarget.Body.Close()

	// Step 3: Honeypot / Content Inspection
	bodyTarget, _ := io.ReadAll(respTarget.Body)
	bodyStr := strings.ToLower(string(bodyTarget))

	if len(bodyTarget) < 2 || strings.Contains(bodyStr, "<html") {
		// The proxy intercepted the traffic and returned a captive portal or error page
		v.store.UpdateScore(proxy.Addr, proxy.Protocol, v.cfg.ScoreDownHoney, 9999)
		return false
	}

	switch respTarget.StatusCode {
	case 200, 400, 401: // Valid business logic responses
		// We expect authentication errors or bad request when hitting without proper body/token
		// Verify shape
		if !bytes.HasPrefix(bytes.TrimSpace(bodyTarget), []byte("{")) {
			v.store.UpdateScore(proxy.Addr, proxy.Protocol, v.cfg.ScoreDownHoney, 9999)
			return false
		}

		v.store.UpdateScore(proxy.Addr, proxy.Protocol, v.cfg.ScoreUp, latency)
		// Mark HTTPS support: validation passed against HTTPS URLs,
		// so the proxy supports HTTPS CONNECT tunneling.
		v.store.SetHTTPS(proxy.Addr, proxy.Protocol)
		v.cb.RecordSuccess()
		return true

	case 403:
		// "Free usage exceeded, subscribe to Go" or CF Blocked
		v.store.DeleteProxy(proxy.Addr, proxy.Protocol)
		v.cb.RecordFailure()
		return false

	case 429:
		// Rate limited temporarily
		v.store.UpdateScore(proxy.Addr, proxy.Protocol, v.cfg.ScoreDown, latency)
		v.cb.RecordFailure()
		return false
	default:
		v.store.UpdateScore(proxy.Addr, proxy.Protocol, v.cfg.ScoreDown, latency)
		return false
	}
}

func (v *Validator) ValidateAll(ctx context.Context) {
	proxies := v.store.GetAll()
	var wg sync.WaitGroup

	for _, p := range proxies {
		wg.Add(1)
		go func(proxy storage.Proxy) {
			defer wg.Done()
			v.ValidateOne(ctx, proxy)
		}(p)
	}

	wg.Wait()
}
