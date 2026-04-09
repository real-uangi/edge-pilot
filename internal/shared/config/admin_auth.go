package config

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

const (
	defaultAdminSessionTTL = 12 * time.Hour
)

type AdminAuthConfig struct {
	Username           string
	Password           string
	SessionSecret      string
	SessionTTL         time.Duration
	CookieName         string
	TrustedProxyCIDRs  []string
	TrustCloudflare    bool
	trustedProxyBlocks []*net.IPNet
}

func LoadAdminAuthConfig() (*AdminAuthConfig, error) {
	cfg := &AdminAuthConfig{
		Username:          strings.TrimSpace(os.Getenv("ADMIN_USERNAME")),
		Password:          os.Getenv("ADMIN_PASSWORD"),
		SessionSecret:     strings.TrimSpace(os.Getenv("ADMIN_SESSION_SECRET")),
		SessionTTL:        defaultAdminSessionTTL,
		CookieName:        "ep_admin_session",
		TrustedProxyCIDRs: parseCSVEnv("TRUSTED_PROXY_CIDRS"),
		TrustCloudflare:   parseBoolEnv("TRUST_CLOUDFLARE"),
	}
	if cfg.Username == "" {
		return nil, fmt.Errorf("ADMIN_USERNAME is required")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("ADMIN_PASSWORD is required")
	}
	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("ADMIN_SESSION_SECRET is required")
	}
	blocks, err := parseProxyBlocks(cfg.TrustedProxyCIDRs)
	if err != nil {
		return nil, err
	}
	cfg.trustedProxyBlocks = blocks
	return cfg, nil
}

func (c *AdminAuthConfig) IsTrustedProxy(remoteAddr string) bool {
	host := remoteAddr
	if parsedHost, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = parsedHost
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return false
	}
	for _, block := range c.trustedProxyBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

func parseCSVEnv(key string) []string {
	raw := os.Getenv(key)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func parseBoolEnv(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseProxyBlocks(items []string) ([]*net.IPNet, error) {
	if len(items) == 0 {
		return nil, nil
	}
	blocks := make([]*net.IPNet, 0, len(items))
	for _, item := range items {
		if _, network, err := net.ParseCIDR(item); err == nil {
			blocks = append(blocks, network)
			continue
		}
		ip := net.ParseIP(item)
		if ip == nil {
			return nil, fmt.Errorf("invalid TRUSTED_PROXY_CIDRS entry: %s", item)
		}
		maskBits := 32
		if ip.To4() == nil {
			maskBits = 128
		}
		blocks = append(blocks, &net.IPNet{IP: ip, Mask: net.CIDRMask(maskBits, maskBits)})
	}
	return blocks, nil
}
