package fetcher

import "context"

// monosans/proxy-list — pre-validated hourly, trust bonus +5.
// https://github.com/monosans/proxy-list
type monosansSource struct{}

func (s *monosansSource) Name() string { return "monosans" }

func (s *monosansSource) Fetch(ctx context.Context) []RawProxy {
	urls := []struct {
		url      string
		protocol string
	}{
		{"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/http.txt", "http"},
		{"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/socks4.txt", "socks4"},
		{"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/socks5.txt", "socks5"},
	}

	var result []RawProxy
	for _, u := range urls {
		body, err := fetchText(ctx, u.url)
		if err != nil {
			continue
		}
		result = append(result, parseProxyLines(body, u.protocol, s.Name(), 5)...)
	}
	return result
}

func init() { Register(&monosansSource{}) }
