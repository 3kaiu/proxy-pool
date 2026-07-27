package fetcher

import (
	"context"
	"log"
)

// ProxyScrape source — text format with protocol:// prefix.
type proxyScrapeSource struct{}

func (s *proxyScrapeSource) Name() string { return "proxyscrape" }

func (s *proxyScrapeSource) Fetch(ctx context.Context) []RawProxy {
	body, err := fetchText(ctx, "https://api.proxyscrape.com/v4/free-proxy-list/get?request=display_proxies&proxy_format=protocolipport&format=text")
	if err != nil {
		log.Printf("[Fetcher] ProxyScrape error: %v", err)
		return nil
	}
	return parseProxyLines(body, "http", s.Name(), 0)
}

func init() { Register(&proxyScrapeSource{}) }
