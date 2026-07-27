package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RawProxy is a proxy discovered by a source, before it enters storage.
type RawProxy struct {
	Addr      string // "ip:port"
	Protocol  string // "http" | "socks4" | "socks5"
	Source    string // source name
	Trust     int    // trust bonus for pre-validated lists
	Anonymity string // "elite" | "anonymous" | "" (from geonode)
	Country   string // "US" | "" (from geonode)
	Uptime    int    // 0-100 (from geonode)
}

// Source is a pluggable proxy source.
type Source interface {
	Name() string
	Fetch(ctx context.Context) []RawProxy
}

var registeredSources []Source
var sourceExclude map[string]bool

// Register adds a source to the registry. Called from each source's init().
func Register(s Source) {
	if sourceExclude != nil && sourceExclude[s.Name()] {
		return
	}
	registeredSources = append(registeredSources, s)
}

// AllSources returns all registered sources.
func AllSources() []Source {
	return registeredSources
}

// SetExclude configures which source names to skip during registration.
func SetExclude(exclude []string) {
	sourceExclude = make(map[string]bool)
	for _, name := range exclude {
		sourceExclude[name] = true
	}
}

// --- Shared HTTP client for all sources ---

var sharedClient = &http.Client{
	Timeout: 15 * time.Second,
}

// SetClient replaces the shared HTTP client (e.g. to add custom UA / timeout).
func SetClient(c *http.Client) {
	sharedClient = c
}

// fetchText fetches a URL and returns the body as a string, with 3 retries.
func fetchText(ctx context.Context, url string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

		resp, err := sharedClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			lastErr = fmt.Errorf("source returned HTTP %s", resp.Status)
			time.Sleep(2 * time.Second)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		return string(body), nil
	}
	return "", lastErr
}

// parseProxyLines parses text with one proxy per line.
// Handles "protocol://ip:port", "ip:port", and "user:pass@ip:port" formats.
// Lines without an explicit protocol use defaultProtocol.
func parseProxyLines(body, defaultProtocol, sourceName string, trust int) []RawProxy {
	var result []RawProxy
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		addr, protocol := parseProxyAddr(line, defaultProtocol)
		if addr == "" {
			continue
		}

		result = append(result, RawProxy{
			Addr:     addr,
			Protocol: protocol,
			Source:   sourceName,
			Trust:    trust,
		})
	}
	return result
}

// parseProxyAddr extracts the address and protocol from a proxy string.
// Handles:
//   - "http://1.2.3.4:8080"  -> ("1.2.3.4:8080", "http")
//   - "socks5://1.2.3.4:1080" -> ("1.2.3.4:1080", "socks5")
//   - "user:pass@1.2.3.4:8080" -> ("1.2.3.4:8080", defaultProto)
//   - "1.2.3.4:8080"          -> ("1.2.3.4:8080", defaultProto)
func parseProxyAddr(raw, defaultProto string) (addr, protocol string) {
	raw = strings.TrimSpace(raw)

	// Handle protocol:// prefix
	if idx := strings.Index(raw, "://"); idx > 0 {
		proto := strings.ToLower(raw[:idx])
		rest := raw[idx+3:]
		// Strip auth prefix if present
		if at := strings.LastIndex(rest, "@"); at >= 0 {
			rest = rest[at+1:]
		}
		return rest, proto
	}

	// Strip auth prefix "user:pass@host:port"
	if at := strings.LastIndex(raw, "@"); at >= 0 {
		raw = raw[at+1:]
	}

	return raw, defaultProto
}
