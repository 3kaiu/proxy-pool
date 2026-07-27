package storage

import (
	"database/sql"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

// Proxy represents a single proxy entry with full metadata.
type Proxy struct {
	Addr      string  `json:"proxy"`     // "ip:port"
	Protocol  string  `json:"protocol"`  // "http" | "socks4" | "socks5"
	Score     int     `json:"score"`
	Latency   int     `json:"latency"`
	Success   int     `json:"success"`
	Fail      int     `json:"fail"`
	Source    string  `json:"source"`    // which source(s) found it
	HTTPS     bool    `json:"https"`     // supports HTTPS CONNECT tunneling
	LastCheck float64 `json:"last_check"`
	Created   float64 `json:"created"`
}

type WriteOp struct {
	Kind     string // "add", "update", "delete", "cleanup", "update_https"
	Proxy    string
	Protocol string
	Source   string
	Delta    int
	Latency  int
	HTTPS    bool
	Created  time.Time
}

type Stats struct {
	Total       int            `json:"total"`
	Available   int            `json:"available"`
	AvgLatency  int            `json:"avg_latency_ms"`
	ByProtocol  map[string]int `json:"by_protocol"`
}

type Storage struct {
	db       *sql.DB
	writeCh  chan WriteOp
	maxScore int
}

func New(dbPath string, queueSize int, maxScore int) (*Storage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`)
	if err != nil {
		return nil, err
	}

	// Drop old single-PK table if it exists (data is ephemeral, re-fetched every 3 min)
	_, _ = db.Exec(`DROP TABLE IF EXISTS proxy`)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS proxy (
			ip_port    TEXT NOT NULL,
			protocol   TEXT NOT NULL DEFAULT 'http',
			score      INTEGER DEFAULT 10,
			latency    INTEGER DEFAULT 9999,
			success    INTEGER DEFAULT 0,
			fail       INTEGER DEFAULT 0,
			source     TEXT DEFAULT '',
			https      INTEGER DEFAULT 0,
			last_check REAL DEFAULT 0,
			created    REAL DEFAULT 0,
			PRIMARY KEY (protocol, ip_port)
		)
	`)
	if err != nil {
		return nil, err
	}

	s := &Storage{
		db:       db,
		writeCh:  make(chan WriteOp, queueSize),
		maxScore: maxScore,
	}

	go s.writerLoop()

	return s, nil
}

func (s *Storage) writerLoop() {
	for op := range s.writeCh {
		switch op.Kind {
		case "add":
			_, err := s.db.Exec(`
				INSERT OR IGNORE INTO proxy(ip_port, protocol, score, source, created)
				VALUES(?, ?, ?, ?, ?)
			`, op.Proxy, op.Protocol, op.Delta, op.Source, float64(op.Created.Unix()))
			if err != nil {
				log.Printf("[Storage] Add error: %v\n", err)
			}

		case "update":
			now := float64(time.Now().Unix())

			var score, success, fail int
			err := s.db.QueryRow(
				"SELECT score, success, fail FROM proxy WHERE ip_port=? AND protocol=?",
				op.Proxy, op.Protocol,
			).Scan(&score, &success, &fail)
			if err != nil {
				continue
			}

			newScore := score + op.Delta
			if newScore > s.maxScore {
				newScore = s.maxScore
			}
			if newScore < 0 {
				newScore = 0
			}

			newSuccess := success
			newFail := fail
			if op.Delta > 0 {
				newSuccess++
			} else if op.Delta < 0 {
				newFail++
			}

			_, err = s.db.Exec(`
				UPDATE proxy SET score=?, latency=?, success=?, fail=?, last_check=?
				WHERE ip_port=? AND protocol=?
			`, newScore, op.Latency, newSuccess, newFail, now, op.Proxy, op.Protocol)

			if err != nil {
				log.Printf("[Storage] Update error: %v\n", err)
			}

		case "update_https":
			_, err := s.db.Exec(`
				UPDATE proxy SET https=1 WHERE ip_port=? AND protocol=?
			`, op.Proxy, op.Protocol)
			if err != nil {
				log.Printf("[Storage] Update HTTPS error: %v\n", err)
			}

		case "delete":
			_, err := s.db.Exec(
				"DELETE FROM proxy WHERE ip_port=? AND protocol=?",
				op.Proxy, op.Protocol,
			)
			if err != nil {
				log.Printf("[Storage] Delete error: %v\n", err)
			}

		case "cleanup":
			_, err := s.db.Exec("DELETE FROM proxy WHERE score <= 0")
			if err != nil {
				log.Printf("[Storage] Cleanup error: %v\n", err)
			}
		}
	}
}

// --- Public API ---

func (s *Storage) AddProxy(addr, protocol, source string, initialScore int) {
	s.writeCh <- WriteOp{
		Kind:     "add",
		Proxy:    addr,
		Protocol: protocol,
		Source:   source,
		Delta:    initialScore,
		Created:  time.Now(),
	}
}

func (s *Storage) UpdateScore(addr, protocol string, delta, latency int) {
	s.writeCh <- WriteOp{
		Kind:     "update",
		Proxy:    addr,
		Protocol: protocol,
		Delta:    delta,
		Latency:  latency,
	}
}

func (s *Storage) SetHTTPS(addr, protocol string) {
	s.writeCh <- WriteOp{
		Kind:     "update_https",
		Proxy:    addr,
		Protocol: protocol,
	}
}

func (s *Storage) DeleteProxy(addr, protocol string) {
	s.writeCh <- WriteOp{
		Kind:     "delete",
		Proxy:    addr,
		Protocol: protocol,
	}
}

func (s *Storage) CleanupDead() {
	s.writeCh <- WriteOp{Kind: "cleanup"}
}

func (s *Storage) GetBest(count int) []Proxy {
	return s.GetBestByProtocol("", count)
}

func (s *Storage) GetBestByProtocol(protocol string, count int) []Proxy {
	var rows *sql.Rows
	var err error

	if protocol == "" {
		rows, err = s.db.Query(`
			SELECT ip_port, protocol, score, latency, source, https
			FROM proxy
			WHERE score > 0
			ORDER BY score DESC, latency ASC
			LIMIT ?
		`, count)
	} else {
		rows, err = s.db.Query(`
			SELECT ip_port, protocol, score, latency, source, https
			FROM proxy
			WHERE score > 0 AND protocol=?
			ORDER BY score DESC, latency ASC
			LIMIT ?
		`, protocol, count)
	}

	if err != nil {
		log.Printf("[Storage] GetBest query error: %v", err)
		return nil
	}
	defer rows.Close()

	return scanProxies(rows)
}

func (s *Storage) PopBest() *Proxy {
	return s.PopBestByProtocol("")
}

func (s *Storage) PopBestByProtocol(protocol string) *Proxy {
	best := s.GetBestByProtocol(protocol, 1)
	if len(best) > 0 {
		p := best[0]
		s.UpdateScore(p.Addr, p.Protocol, -2, 0)
		return &p
	}
	return nil
}

func (s *Storage) GetAll() []Proxy {
	rows, err := s.db.Query(`
		SELECT ip_port, protocol, score, latency, source, https
		FROM proxy
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanProxies(rows)
}

func (s *Storage) Count() int {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM proxy").Scan(&count)
	return count
}

func (s *Storage) CountByProtocol() map[string]int {
	rows, err := s.db.Query("SELECT protocol, COUNT(*) FROM proxy GROUP BY protocol")
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var proto string
		var count int
		if err := rows.Scan(&proto, &count); err == nil {
			result[proto] = count
		}
	}
	return result
}

func (s *Storage) Stats() Stats {
	var total, available, latencySum, latCount int

	s.db.QueryRow("SELECT COUNT(*) FROM proxy").Scan(&total)
	s.db.QueryRow("SELECT COUNT(*) FROM proxy WHERE score > 3").Scan(&available)
	s.db.QueryRow("SELECT SUM(latency), COUNT(latency) FROM proxy WHERE score > 3").Scan(&latencySum, &latCount)

	avgLat := 0
	if latCount > 0 {
		avgLat = latencySum / latCount
	}

	return Stats{
		Total:      total,
		Available:  available,
		AvgLatency: avgLat,
		ByProtocol: s.CountByProtocol(),
	}
}

func (s *Storage) Close() error {
	close(s.writeCh)
	return s.db.Close()
}

// --- helpers ---

func scanProxies(rows *sql.Rows) []Proxy {
	var proxies []Proxy
	for rows.Next() {
		var p Proxy
		var https int
		if err := rows.Scan(&p.Addr, &p.Protocol, &p.Score, &p.Latency, &p.Source, &https); err == nil {
			p.HTTPS = https == 1
			proxies = append(proxies, p)
		}
	}
	return proxies
}
