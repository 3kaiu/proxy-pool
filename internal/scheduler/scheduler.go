package scheduler

import (
	"context"
	"log"
	"time"

	"proxy-pool/internal/config"
	"proxy-pool/internal/fetcher"
	"proxy-pool/internal/storage"
	"proxy-pool/internal/validator"
)

type Scheduler struct {
	cfg       config.Config
	store     *storage.Storage
	fetcher   *fetcher.Fetcher
	validator *validator.Validator
}

func New(cfg config.Config, store *storage.Storage, fetcher *fetcher.Fetcher, validator *validator.Validator) *Scheduler {
	return &Scheduler{
		cfg:       cfg,
		store:     store,
		fetcher:   fetcher,
		validator: validator,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	go s.runFetcher(ctx)
	go s.runValidator(ctx)
	go s.runCleanup(ctx)
}

// runFetchOnce fetches proxies from all sources and adds them to storage.
// Shared by scheduled fetch and low-water-mark emergency fetch.
func (s *Scheduler) runFetchOnce(ctx context.Context) {
	proxies := s.fetcher.FetchAll(ctx)
	log.Printf("[Scheduler] Fetched %d unique proxies", len(proxies))

	for _, p := range proxies {
		initialScore := s.cfg.InitialScore + p.Trust
		s.store.AddProxy(p.Addr, p.Protocol, p.Source, initialScore)
	}
}

func (s *Scheduler) runFetcher(ctx context.Context) {
	// Fetch immediately on startup
	log.Println("[Scheduler] Starting fetch cycle")
	s.runFetchOnce(ctx)

	ticker := time.NewTicker(s.cfg.FetchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("[Scheduler] Starting fetch cycle")
			s.runFetchOnce(ctx)
		}
	}
}

func (s *Scheduler) runValidator(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.ValidateInterval)
	defer ticker.Stop()

	for {
		// Low-water-mark check: if pool is too small, trigger emergency fetch
		if s.store.Count() < s.cfg.PoolSizeMin {
			log.Printf("[Scheduler] Pool below minimum (%d < %d), triggering emergency fetch",
				s.store.Count(), s.cfg.PoolSizeMin)
			s.runFetchOnce(ctx)
		}

		log.Println("[Scheduler] Starting validation cycle")
		s.validator.ValidateAll(ctx)
		log.Println("[Scheduler] Validation cycle finished")

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Scheduler) runCleanup(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.CleanupInterval)
	defer ticker.Stop()

	for {
		log.Println("[Scheduler] Starting cleanup cycle")
		s.store.CleanupDead()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
