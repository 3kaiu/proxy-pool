package server

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	http_f "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const zenBaseURL = "https://opencode.ai/zen"
const relayMaxRetries = 30
const relayTimeoutSec = 20

// handleRelay proxies OpenAI-compatible requests through the proxy pool.
// e.g. POST /v1/chat/completions -> https://opencode.ai/zen/v1/chat/completions
//      GET  /v1/models            -> https://opencode.ai/zen/v1/models
func (s *Server) handleRelay(w http.ResponseWriter, r *http.Request) {
	// Build target URL
	targetURL := zenBaseURL + r.URL.Path
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	// Read request body
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		r.Body.Close()
	}

	// Only forward essential headers (copying all headers causes 400 from Cloudflare)
	forwardHeaders := http_f.Header{}
	essential := map[string]bool{
		"content-type":  true,
		"authorization": true,
		"accept":        true,
	}
	for key, vals := range r.Header {
		if essential[strings.ToLower(key)] {
			for _, v := range vals {
				forwardHeaders.Add(key, v)
			}
		}
	}
	forwardHeaders.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	forwardHeaders.Set("Accept", "application/json")

	var lastErr string
	var lastStatus int

	for attempt := 0; attempt < relayMaxRetries; attempt++ {
		// Get a proxy from the pool
		protocol := r.URL.Query().Get("protocol") // optional filter
		pi := s.store.PopBestByProtocol(protocol)
		if pi == nil {
			lastErr = "no proxy available in pool"
			time.Sleep(500 * time.Millisecond)
			continue
		}

		proxyURL := pi.Protocol + "://" + pi.Addr

		tlsClient, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(),
			tls_client.WithTimeoutSeconds(relayTimeoutSec),
			tls_client.WithClientProfile(profiles.Chrome_124),
			tls_client.WithProxyUrl(proxyURL),
		)
		if err != nil {
			s.store.DeleteProxy(pi.Addr, pi.Protocol)
			continue
		}

		var reqBody io.Reader
		if len(body) > 0 {
			reqBody = bytes.NewReader(body)
		}

		req, err := http_f.NewRequest(r.Method, targetURL, reqBody)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		req.Header = forwardHeaders

		resp, err := tlsClient.Do(req)
		if err != nil {
			s.store.DeleteProxy(pi.Addr, pi.Protocol)
			lastErr = err.Error()
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Delete dead proxies (5xx or connection errors)
		if resp.StatusCode >= 500 {
			s.store.DeleteProxy(pi.Addr, pi.Protocol)
			lastStatus = resp.StatusCode
			lastErr = "upstream 5xx"
			continue
		}

		// Check for rate limit in body
		bodyStr := string(respBody)
		if strings.Contains(bodyStr, "FreeUsageLimitError") || strings.Contains(bodyStr, "rate_limit") {
			// This proxy's IP is rate limited, try another
			lastErr = "rate_limited"
			lastStatus = resp.StatusCode
			continue
		}

		// Success - copy response headers and body
		for key, vals := range resp.Header {
			for _, v := range vals {
				w.Header().Add(key, v)
			}
		}
		w.Header().Set("X-Proxy-Used", pi.Addr)
		w.Header().Set("X-Proxy-Protocol", pi.Protocol)
		w.Header().Set("X-Retries", strconv.Itoa(attempt+1))
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)

		log.Printf("[Relay] %s %s -> %d (proxy=%s, attempts=%d)",
			r.Method, r.URL.Path, resp.StatusCode, proxyURL, attempt+1)
		return
	}

	// All retries failed
	log.Printf("[Relay] %s %s -> FAILED after %d attempts: %s",
		r.Method, r.URL.Path, relayMaxRetries, lastErr)

	w.Header().Set("Content-Type", "application/json")
	statusCode := http.StatusBadGateway
	if lastStatus > 0 {
		statusCode = lastStatus
	}
	w.WriteHeader(statusCode)
	w.Write([]byte(`{"error":{"message":"All proxy retries failed: ` + lastErr + `","type":"proxy_pool_error","code":` + string(rune('0'+statusCode)) + `}}`))
}
