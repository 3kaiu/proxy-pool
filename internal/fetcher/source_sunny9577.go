package fetcher

import "context"

// sunny9577/proxy-scraper source — plain text list.
type sunny9577Source struct{}

func (s *sunny9577Source) Name() string { return "sunny9577" }

func (s *sunny9577Source) Fetch(ctx context.Context) []RawProxy {
	body, err := fetchText(ctx, "https://raw.githubusercontent.com/sunny9577/proxy-scraper/master/proxies.txt")
	if err != nil {
		return nil
	}
	return parseProxyLines(body, "http", s.Name(), 0)
}

func init() { Register(&sunny9577Source{}) }
