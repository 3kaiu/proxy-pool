package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TargetURL              string
	TargetHost             string
	APIKey                 string
	IPCheckURLs            []string
	BlockedCIDRs           []string
	FetchInterval          time.Duration
	ValidateInterval       time.Duration
	CleanupInterval        time.Duration
	MaxValidateConcurrency int
	ValidateTimeout        time.Duration
	InitialScore           int
	ScoreUp                int
	ScoreDown              int
	ScoreDownSSL           int
	ScoreDownHoney         int
	MaxScore               int
	CBThreshold            int
	CBCooldown             time.Duration
	PaddingEnabled         bool
	PaddingMin             int
	PaddingMax             int
	ListenAddr             string
	DBPath                 string
	DBWriteQueueSize       int
	PoolSizeMin            int
	SourceExclude          []string
}

var DefaultConfig = Config{
	TargetURL:  "https://opencode.ai/zen/v1/models",
	TargetHost: "opencode.ai",
	APIKey:     "", // can be set via PROXY_POOL_API_KEY
	IPCheckURLs: []string{
		"https://api.ipify.org?format=json",
		"https://httpbin.org/ip",
		"https://ifconfig.me/all.json",
		"https://api.ip.sb/jsonip",
		"https://ipapi.co/json/",
	},
	BlockedCIDRs: []string{
		"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"127.0.0.0/8", "169.254.0.0/16", "0.0.0.0/8",
		"100.64.0.0/10", "::1/128", "fc00::/7",
	},
	FetchInterval:          3 * time.Minute,
	ValidateInterval:       5 * time.Minute,
	CleanupInterval:        30 * time.Minute,
	MaxValidateConcurrency: 200,
	ValidateTimeout:        8 * time.Second,
	InitialScore:           10,
	ScoreUp:                1,
	ScoreDown:              -1,
	ScoreDownSSL:           -3,
	ScoreDownHoney:         -5,
	MaxScore:               100,
	CBThreshold:            10,
	CBCooldown:             5 * time.Minute,
	PaddingEnabled:         true,
	PaddingMin:             64,
	PaddingMax:             512,
	ListenAddr:             "127.0.0.1:5010",
	DBPath:                 "proxies.db",
	DBWriteQueueSize:       5000,
	PoolSizeMin:            20,
	SourceExclude:          []string{},
}

func LoadConfig() Config {
	cfg := DefaultConfig

	if val := os.Getenv("PROXY_POOL_TARGET_URL"); val != "" {
		cfg.TargetURL = val
	}
	if val := os.Getenv("PROXY_POOL_TARGET_HOST"); val != "" {
		cfg.TargetHost = val
	}
	if val := os.Getenv("PROXY_POOL_DB_PATH"); val != "" {
		cfg.DBPath = val
	}
	if val := os.Getenv("PROXY_POOL_API_KEY"); val != "" {
		cfg.APIKey = val
	}
	if val := os.Getenv("PROXY_POOL_LISTEN"); val != "" {
		cfg.ListenAddr = val
	}
	if val := os.Getenv("PROXY_POOL_MAX_CONCURRENCY"); val != "" {
		if c, err := strconv.Atoi(val); err == nil {
			cfg.MaxValidateConcurrency = c
		}
	}
	if val := os.Getenv("PROXY_POOL_IP_CHECK_URLS"); val != "" {
		urls := strings.Split(val, ",")
		for i, u := range urls {
			urls[i] = strings.TrimSpace(u)
		}
		cfg.IPCheckURLs = urls
	}
	if val := os.Getenv("PROXY_POOL_POOL_SIZE_MIN"); val != "" {
		if c, err := strconv.Atoi(val); err == nil {
			cfg.PoolSizeMin = c
		}
	}
	if val := os.Getenv("PROXY_POOL_SOURCE_EXCLUDE"); val != "" {
		excludes := strings.Split(val, ",")
		for i, s := range excludes {
			excludes[i] = strings.TrimSpace(s)
		}
		cfg.SourceExclude = excludes
	}

	return cfg
}
