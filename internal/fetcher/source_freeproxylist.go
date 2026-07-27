package fetcher

import (
	"context"
	"log"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// free-proxy-list.net source — HTML table scraper.
type freeProxyListSource struct{}

func (s *freeProxyListSource) Name() string { return "freeproxylist" }

func (s *freeProxyListSource) Fetch(ctx context.Context) []RawProxy {
	body, err := fetchText(ctx, "https://free-proxy-list.net/")
	if err != nil {
		log.Printf("[Fetcher] free-proxy-list.net error: %v", err)
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil
	}

	var result []RawProxy
	doc.Find("table tbody tr").Each(func(i int, sel *goquery.Selection) {
		ip := strings.TrimSpace(sel.Find("td:nth-child(1)").Text())
		port := strings.TrimSpace(sel.Find("td:nth-child(2)").Text())
		if ip != "" && port != "" {
			result = append(result, RawProxy{
				Addr:     ip + ":" + port,
				Protocol: "http",
				Source:   s.Name(),
			})
		}
	})
	return result
}

func init() { Register(&freeProxyListSource{}) }
