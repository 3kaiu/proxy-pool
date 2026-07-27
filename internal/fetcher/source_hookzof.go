package fetcher

import "context"

// hookzof/socks5_list — SOCKS5-only proxy list.
// https://github.com/hookzof/socks5_list
type hookzofSource struct{}

func (s *hookzofSource) Name() string { return "hookzof" }

func (s *hookzofSource) Fetch(ctx context.Context) []RawProxy {
	body, err := fetchText(ctx, "https://raw.githubusercontent.com/hookzof/socks5_list/master/proxy.txt")
	if err != nil {
		return nil
	}
	return parseProxyLines(body, "socks5", s.Name(), 1)
}

func init() { Register(&hookzofSource{}) }
