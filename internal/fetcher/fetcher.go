package fetcher

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Fetcher struct {
	client *http.Client
}

func New() *Fetcher {
	c := &http.Client{
		Timeout: 15 * time.Second,
	}
	SetClient(c)
	return &Fetcher{client: c}
}

// FetchAll fetches proxies from all registered sources concurrently.
// Returns deduplicated []RawProxy with composite (protocol, addr) keys.
func (f *Fetcher) FetchAll(ctx context.Context) []RawProxy {
	sources := AllSources()
	if len(sources) == 0 {
		log.Println("[Fetcher] No sources registered")
		return nil
	}

	proxyChan := make(chan RawProxy, 10000)

	var wg sync.WaitGroup
	for _, src := range sources {
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			log.Printf("[Fetcher] Fetching from %s", s.Name())
			proxies := s.Fetch(ctx)
			log.Printf("[Fetcher] %s returned %d proxies", s.Name(), len(proxies))
			for _, p := range proxies {
				proxyChan <- p
			}
		}(src)
	}

	go func() {
		wg.Wait()
		close(proxyChan)
	}()

	// Dedup on (protocol, addr) composite key.
	// Track multi-source: if same proxy found by multiple sources, join names with "/".
	type dedupEntry struct {
		proxy    RawProxy
		sourceSB strings.Builder
		count    int
	}
	seen := make(map[string]*dedupEntry)

	for p := range proxyChan {
		if p.Addr == "" || p.Protocol == "" {
			continue
		}
		if !IsSafeProxy(p.Addr) {
			continue
		}

		key := p.Protocol + "|" + p.Addr
		if entry, exists := seen[key]; exists {
			// Multi-source: join source names, take max trust
			entry.sourceSB.WriteString("/")
			entry.sourceSB.WriteString(p.Source)
			if p.Trust > entry.proxy.Trust {
				entry.proxy.Trust = p.Trust
			}
			entry.count++
		} else {
			entry := &dedupEntry{
				proxy: p,
				count: 1,
			}
			entry.sourceSB.WriteString(p.Source)
			seen[key] = entry
		}
	}

	result := make([]RawProxy, 0, len(seen))
	for _, entry := range seen {
		entry.proxy.Source = entry.sourceSB.String()
		// Multi-source bonus: +1 trust per additional source
		if entry.count > 1 {
			entry.proxy.Trust += entry.count - 1
		}
		result = append(result, entry.proxy)
	}

	log.Printf("[Fetcher] Total unique proxies after dedup: %d", len(result))
	return result
}
