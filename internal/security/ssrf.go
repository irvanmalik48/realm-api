package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

var (
	ErrPrivateIPBlocked = errors.New("connection to private/internal IP address is blocked")
	ErrInvalidScheme    = errors.New("only HTTPS scheme is allowed for external webhooks")
	ErrDisallowedHost   = errors.New("target host is not in the allowed webhook domain whitelist")
)

var privateIPBlocks []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",          // IPv4 loopback
		"10.0.0.0/8",           // RFC1918
		"172.16.0.0/12",        // RFC1918
		"192.168.0.0/16",       // RFC1918
		"169.254.0.0/16",       // Link-local / Cloud metadata
		"0.0.0.0/8",            // Current network
		"100.64.0.0/10",        // Carrier-grade NAT
		"198.18.0.0/15",        // Benchmark testing
		"::1/128",              // IPv6 loopback
		"fc00::/7",             // IPv6 unique local
		"fe80::/10",            // IPv6 link-local
	} {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			privateIPBlocks = append(privateIPBlocks, block)
		}
	}
}

// IsPrivateIP checks if an IP is private, loopback, or link-local.
func IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// SafeDialer returns a dialer that prevents connections to private or loopback IPs (SSRF protection).
func SafeDialControl() func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			host = address
		}

		ip := net.ParseIP(host)
		if ip != nil && IsPrivateIP(ip) {
			return ErrPrivateIPBlocked
		}
		return nil
	}
}

// NewSafeHTTPClient creates an HTTP client protected against SSRF and open redirect attacks.
func NewSafeHTTPClient(timeout time.Duration) *http.Client {
	netDialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Control:   SafeDialControl(),
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}

			for _, ip := range ips {
				if IsPrivateIP(ip.IP) {
					return nil, fmt.Errorf("%w: %s (%s)", ErrPrivateIPBlocked, host, ip.IP)
				}
			}

			return netDialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
		MaxIdleConns:          50,
		IdleConnTimeout:       60 * time.Second,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		// Block automated open redirects to internal targets
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// ValidateDiscordWebhookURL ensures the webhook URL is a valid, secure Discord webhook.
func ValidateDiscordWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	if u.Scheme != "https" {
		return ErrInvalidScheme
	}

	host := strings.ToLower(u.Hostname())
	if host != "discord.com" && host != "discordapp.com" && !strings.HasSuffix(host, ".discord.com") {
		return fmt.Errorf("%w: %s", ErrDisallowedHost, host)
	}

	if !strings.HasPrefix(u.Path, "/api/webhooks/") {
		return errors.New("invalid discord webhook path")
	}

	return nil
}

// ValidateTelegramBotToken ensures token format is valid.
func ValidateTelegramBotToken(token string) bool {
	// Telegram bot tokens are formatted like 123456789:ABCdefGhIJKlmNoPQRsTUVwxyZ
	parts := strings.Split(token, ":")
	return len(parts) == 2 && len(parts[0]) > 0 && len(parts[1]) > 10
}
