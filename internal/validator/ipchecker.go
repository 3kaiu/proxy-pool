package validator

import (
	"math/rand"
	"net/url"
	"time"
)

type IPChecker struct {
	sources []string
	fields  map[string]string
}

func NewIPChecker(urls []string) *IPChecker {
	rand.Seed(time.Now().UnixNano())
	
	fields := map[string]string{
		"api.ipify.org":  "ip",
		"httpbin.org":    "origin",
		"ifconfig.me":    "ip_addr",
		"api.ip.sb":      "ip",
		"ipapi.co":       "ip",
	}

	return &IPChecker{
		sources: urls,
		fields:  fields,
	}
}

func (c *IPChecker) Pick() (string, string) {
	if len(c.sources) == 0 {
		return "https://httpbin.org/ip", "origin"
	}
	u := c.sources[rand.Intn(len(c.sources))]
	parsed, err := url.Parse(u)
	field := "ip" // default
	if err == nil {
		if f, ok := c.fields[parsed.Host]; ok {
			field = f
		}
	}
	return u, field
}
