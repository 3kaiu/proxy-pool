package fetcher

import (
	"context"
	"encoding/json"
	"log"
)

// geonode source — JSON API with metadata (anonymity, uptime, country).
// Pre-filters to elite proxies with uptime >= 80% to cut ~60% of bad proxies.
// https://proxylist.geonode.com
type geonodeSource struct{}

func (s *geonodeSource) Name() string { return "geonode" }

type geonodeResponse struct {
	Data []struct {
		IP             string   `json:"ip"`
		Port           string   `json:"port"`
		Protocols      []string `json:"protocols"`
		AnonymityLevel string   `json:"anonymityLevel"`
		UpTime         float64  `json:"upTime"`
		Country        string   `json:"country"`
	} `json:"data"`
}

func (s *geonodeSource) Fetch(ctx context.Context) []RawProxy {
	url := "https://proxylist.geonode.com/api/proxy-list?limit=500&page=1&sort_by=lastChecked&sort_type=desc"
	body, err := fetchText(ctx, url)
	if err != nil {
		log.Printf("[Fetcher] geonode error: %v", err)
		return nil
	}

	var resp geonodeResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		log.Printf("[Fetcher] geonode parse error: %v", err)
		return nil
	}

	var result []RawProxy
	for _, item := range resp.Data {
		// Pre-filter: only elite proxies with decent uptime
		if item.AnonymityLevel != "elite" || item.UpTime < 80 {
			continue
		}
		if item.IP == "" || item.Port == "" {
			continue
		}

		protocol := "http"
		if len(item.Protocols) > 0 {
			protocol = item.Protocols[0]
		}

		result = append(result, RawProxy{
			Addr:      item.IP + ":" + item.Port,
			Protocol:  protocol,
			Source:    s.Name(),
			Trust:     2,
			Anonymity: item.AnonymityLevel,
			Country:   item.Country,
			Uptime:    int(item.UpTime),
		})
	}
	return result
}

func init() { Register(&geonodeSource{}) }
