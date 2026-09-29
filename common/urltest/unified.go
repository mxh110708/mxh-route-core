package urltest

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
)

type unifiedDelayKey struct{}

// WithUnifiedDelay opts manual desktop URL tests into Mihomo-style warmed
// HTTP latency. Background health probes retain their existing cold test.
func WithUnifiedDelay(ctx context.Context) context.Context {
	return context.WithValue(ctx, unifiedDelayKey{}, true)
}

func unifiedURLTest(ctx context.Context, link string, detour N.Dialer) (uint16, error) {
	if link == "" {
		link = "https://www.gstatic.com/generate_204"
	}
	ctx, cancel := context.WithTimeout(ctx, C.TCPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, link, nil)
	if err != nil {
		return 0, err
	}
	client := &http.Client{
		Transport: &http.Transport{
			// Always use the complete outbound, including any configured detour.
			// A server closing keep-alive may cause a fresh dial; never bypass it.
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return detour.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
			TLSClientConfig: &tls.Config{
				Time:    ntp.TimeFuncFromContext(ctx),
				RootCAs: adapter.RootPoolFromContext(ctx),
			},
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	coldDuration := time.Since(start)
	start = time.Now()
	resp, err = client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		// A peer may not support a second HEAD. Preserve the successful cold
		// measurement, not the time spent waiting for the failed second request.
		return latencyMilliseconds(coldDuration), nil
	}
	resp.Body.Close()
	return latencyMilliseconds(time.Since(start)), nil
}

func latencyMilliseconds(elapsed time.Duration) uint16 {
	// Zero means unavailable to dashboard/history consumers; avoid overflow.
	return uint16(min(max(elapsed.Milliseconds(), 1), 65535))
}
