package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	http_f "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

type Config struct {
	PoolURL        string // e.g., "http://127.0.0.1:5010"
	PoolAPIKey     string
	PaddingEnabled bool
	PaddingMin     int
	PaddingMax     int
	ProtocolFilter string // "" = any, "http" | "socks4" | "socks5"
}

type Client struct {
	cfg        Config
	poolClient *http.Client
}

func New(cfg Config) *Client {
	if cfg.PaddingMin == 0 {
		cfg.PaddingMin = 64
	}
	if cfg.PaddingMax == 0 {
		cfg.PaddingMax = 512
	}
	return &Client{
		cfg: cfg,
		poolClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type proxyInfo struct {
	Addr     string
	Protocol string
}

func (c *Client) getProxy() *proxyInfo {
	popURL := c.cfg.PoolURL + "/pop"
	if c.cfg.ProtocolFilter != "" {
		popURL += "?protocol=" + url.QueryEscape(c.cfg.ProtocolFilter)
	}

	req, _ := http.NewRequest("GET", popURL, nil)
	if c.cfg.PoolAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.PoolAPIKey)
	}

	resp, err := c.poolClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil
	}

	addr, _ := data["proxy"].(string)
	if addr == "" {
		return nil
	}

	protocol, _ := data["protocol"].(string)
	if protocol == "" {
		protocol = "http" // backward compat with old pool
	}

	return &proxyInfo{Addr: addr, Protocol: protocol}
}

func (c *Client) deleteProxy(addr, protocol string) {
	delURL := c.cfg.PoolURL + "/delete?proxy=" + url.QueryEscape(addr) + "&protocol=" + url.QueryEscape(protocol)
	req, _ := http.NewRequest("GET", delURL, nil)
	if c.cfg.PoolAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.PoolAPIKey)
	}
	c.poolClient.Do(req)
}

func (c *Client) Request(method, url string, headers map[string]string, body []byte) ([]byte, error) {
	rand.Seed(time.Now().UnixNano())

	for attempt := 0; attempt < 30; attempt++ {
		pi := c.getProxy()
		if pi == nil {
			time.Sleep(2 * time.Second)
			continue
		}

		if c.cfg.PaddingEnabled && len(body) > 0 {
			body = c.addPadding(body)
		}

		// Build proxy URL with correct protocol scheme
		proxyURL := pi.Protocol + "://" + pi.Addr

		options := []tls_client.HttpClientOption{
			tls_client.WithTimeoutSeconds(30),
			tls_client.WithClientProfile(profiles.Chrome_124),
			tls_client.WithProxyUrl(proxyURL),
		}

		tlsClient, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
		if err != nil {
			c.deleteProxy(pi.Addr, pi.Protocol)
			continue
		}

		req, err := http_f.NewRequest(method, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}

		for k, v := range headers {
			req.Header.Set(k, v)
		}
		// Enforce chrome-like headers if missing
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
		}

		resp, err := tlsClient.Do(req)
		if err != nil {
			c.deleteProxy(pi.Addr, pi.Protocol)
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 500 {
			c.deleteProxy(pi.Addr, pi.Protocol)
			continue
		}

		if !c.validateResponseFormat(respBody) {
			c.deleteProxy(pi.Addr, pi.Protocol)
			continue
		}

		return respBody, nil
	}

	return nil, errors.New("max retries exceeded: no valid proxy available")
}

func (c *Client) addPadding(body []byte) []byte {
	// Simple JSON padding by adding a dummy field if body is JSON object
	if len(body) > 0 && body[0] == '{' && body[len(body)-1] == '}' {
		padLen := rand.Intn(c.cfg.PaddingMax-c.cfg.PaddingMin+1) + c.cfg.PaddingMin
		padStr := make([]byte, padLen)
		for i := range padStr {
			padStr[i] = 'a' + byte(rand.Intn(26))
		}

		// Insert before the last '}'
		var paddedBody bytes.Buffer
		paddedBody.Write(body[:len(body)-1])
		if len(body) > 2 { // Not just "{}"
			paddedBody.WriteString(",")
		}
		paddedBody.WriteString(`"_pad":"`)
		paddedBody.Write(padStr)
		paddedBody.WriteString(`"}`)
		return paddedBody.Bytes()
	}
	return body
}

func (c *Client) validateResponseFormat(body []byte) bool {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return false
	}

	_, hasChoices := data["choices"]
	_, hasData := data["data"]
	_, hasError := data["error"]

	return hasChoices || hasData || hasError
}
