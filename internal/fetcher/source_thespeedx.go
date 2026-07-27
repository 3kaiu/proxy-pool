package fetcher

import "context"

// TheSpeedX source — three separate files for http/socks4/socks5.
type theSpeedXSource struct{}

func (s *theSpeedXSource) Name() string { return "thespeedx" }

func (s *theSpeedXSource) Fetch(ctx context.Context) []RawProxy {
	urls := []struct {
		url      string
		protocol string
	}{
		{"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt", "http"},
		{"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks4.txt", "socks4"},
		{"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks5.txt", "socks5"},
	}

	var result []RawProxy
	for _, u := range urls {
		body, err := fetchText(ctx, u.url)
		if err != nil {
			continue
		}
		result = append(result, parseProxyLines(body, u.protocol, s.Name(), 0)...)
	}
	return result
}

func init() { Register(&theSpeedXSource{}) }
