package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"proxy-pool/internal/config"
	"proxy-pool/internal/storage"
)

type Server struct {
	cfg   config.Config
	store *storage.Storage
}

func New(cfg config.Config, store *storage.Storage) *Server {
	return &Server{
		cfg:   cfg,
		store: store,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/get", s.handleGet)
	mux.HandleFunc("/pop", s.handlePop)
	mux.HandleFunc("/delete", s.handleDelete)
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/all", s.handleAll)
	mux.HandleFunc("/count", s.handleCount)
	mux.HandleFunc("/v1/", s.handleRelay) // OpenAI-compatible relay through proxy pool

	// Auth middleware if APIKey is set
	var handler http.Handler = mux
	if s.cfg.APIKey != "" {
		handler = s.authMiddleware(mux)
	}

	return http.ListenAndServe(s.cfg.ListenAddr, handler)
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Authorization")
		if key != "Bearer "+s.cfg.APIKey && r.URL.Query().Get("key") != s.cfg.APIKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	countStr := r.URL.Query().Get("count")
	count := 1
	if c, err := strconv.Atoi(countStr); err == nil && c > 0 {
		count = c
	}

	protocol := r.URL.Query().Get("protocol")
	proxies := s.store.GetBestByProtocol(protocol, count)

	// Build response with protocol field
	type proxyResp struct {
		Proxy    string `json:"proxy"`
		Protocol string `json:"protocol"`
		Score    int    `json:"score"`
		HTTPS    bool   `json:"https"`
		Latency  int    `json:"latency"`
	}

	result := make([]proxyResp, 0, len(proxies))
	for _, p := range proxies {
		result = append(result, proxyResp{
			Proxy:    p.Addr,
			Protocol: p.Protocol,
			Score:    p.Score,
			HTTPS:    p.HTTPS,
			Latency:  p.Latency,
		})
	}

	s.jsonResponse(w, map[string]interface{}{
		"proxies": result,
	})
}

func (s *Server) handlePop(w http.ResponseWriter, r *http.Request) {
	protocol := r.URL.Query().Get("protocol")
	p := s.store.PopBestByProtocol(protocol)
	if p == nil {
		s.jsonResponse(w, map[string]interface{}{
			"proxy":    "",
			"protocol": "",
		})
		return
	}
	s.jsonResponse(w, map[string]interface{}{
		"proxy":    p.Addr,
		"protocol": p.Protocol,
		"score":    p.Score,
		"https":    p.HTTPS,
	})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	proxy := r.URL.Query().Get("proxy")
	if proxy == "" {
		http.Error(w, "Missing proxy parameter", http.StatusBadRequest)
		return
	}
	protocol := r.URL.Query().Get("protocol")
	if protocol == "" {
		protocol = "http" // backward compat
	}
	s.store.DeleteProxy(proxy, protocol)
	s.jsonResponse(w, map[string]string{"status": "deleted"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.store.Stats()
	s.jsonResponse(w, stats)
}

func (s *Server) handleAll(w http.ResponseWriter, r *http.Request) {
	protocol := r.URL.Query().Get("protocol")

	countStr := r.URL.Query().Get("count")
	count := 0 // 0 = no limit
	if c, err := strconv.Atoi(countStr); err == nil && c > 0 {
		count = c
	}

	var proxies []storage.Proxy
	if protocol == "" {
		proxies = s.store.GetAll()
	} else {
		proxies = s.store.GetBestByProtocol(protocol, 999999)
	}

	if count > 0 && len(proxies) > count {
		proxies = proxies[:count]
	}

	type proxyResp struct {
		Proxy    string `json:"proxy"`
		Protocol string `json:"protocol"`
		Score    int    `json:"score"`
		HTTPS    bool   `json:"https"`
		Latency  int    `json:"latency"`
	}

	result := make([]proxyResp, 0, len(proxies))
	for _, p := range proxies {
		result = append(result, proxyResp{
			Proxy:    p.Addr,
			Protocol: p.Protocol,
			Score:    p.Score,
			HTTPS:    p.HTTPS,
			Latency:  p.Latency,
		})
	}
	s.jsonResponse(w, result)
}

func (s *Server) handleCount(w http.ResponseWriter, r *http.Request) {
	count := s.store.Count()
	byProtocol := s.store.CountByProtocol()

	stats := s.store.Stats()
	s.jsonResponse(w, map[string]interface{}{
		"total":      count,
		"available":  stats.Available,
		"by_protocol": byProtocol,
	})
}

func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
