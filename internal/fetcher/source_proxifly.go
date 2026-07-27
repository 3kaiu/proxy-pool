package fetcher

import "context"

// proxifly/free-proxy-list — pre-validated every 5 min, served via jsDelivr CDN.
// Trust bonus +3.
// https://github.com/proxifly/free-proxy-list
type proxiflySource struct{}

func (s *proxiflySource) Name() string { return "proxifly" }

func (s *proxiflySource) Fetch(ctx context.Context) []RawProxy {
	urls := []struct {
		url      string
		protocol string
	}{
		{"https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/protocols/http/data.txt", "http"},
		{"https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/protocols/socks4/data.txt", "socks4"},
		{"https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/protocols/socks5/data.txt", "socks5"},
	}

	var result []RawProxy
	for _, u := range urls {
		body, err := fetchText(ctx, u.url)
		if err != nil {
			continue
		}
		result = append(result, parseProxyLines(body, u.protocol, s.Name(), 3)...)
	}
	return result
}

func init() { Register(&proxiflySource{}) }
