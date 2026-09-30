package urltest

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	M "github.com/sagernet/sing/common/metadata"
)

type latencyTestDialer struct {
	address string
	dials   atomic.Int32
	delay   time.Duration
	fail    bool
}

func (d *latencyTestDialer) DialContext(ctx context.Context, network string, _ M.Socksaddr) (net.Conn, error) {
	d.dials.Add(1)
	if d.fail {
		return nil, errors.New("entry unavailable")
	}
	select {
	case <-time.After(d.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return (&net.Dialer{}).DialContext(ctx, network, d.address)
}

func (*latencyTestDialer) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("unexpected UDP")
}

func TestUnifiedDelayDefaultTarget(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.Host != "cp.cloudflare.com" || r.URL.RequestURI() != "/generate_204" {
			t.Errorf("unexpected default probe: %s %s %s", r.Method, r.Host, r.URL.RequestURI())
		}
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	d := &latencyTestDialer{address: server.Listener.Addr().String()}
	_, err := URLTest(WithUnifiedDelay(context.Background()), "", d)
	if err != nil || requests.Load() != 2 || d.dials.Load() != 1 {
		t.Fatalf("default probe: requests=%d dials=%d err=%v", requests.Load(), d.dials.Load(), err)
	}
}

func TestUnifiedDelayReusesFullOutbound(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Error("expected HEAD")
		}
		if requests.Add(1) == 1 {
			time.Sleep(200 * time.Millisecond)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	d := &latencyTestDialer{address: server.Listener.Addr().String(), delay: 200 * time.Millisecond}
	// Deliberately unresolvable: only the supplied outbound can reach the server.
	delay, err := URLTest(WithUnifiedDelay(context.Background()), "http://landing.invalid/probe", d)
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 || d.dials.Load() != 1 {
		t.Fatalf("requests=%d dials=%d", requests.Load(), d.dials.Load())
	}
	if delay == 0 || delay >= 200 {
		t.Fatalf("warm latency includes cold setup: %d", delay)
	}
}

func TestColdHealthProbeUnchanged(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	d := &latencyTestDialer{address: server.Listener.Addr().String(), delay: 100 * time.Millisecond}
	delay, err := URLTest(context.Background(), server.URL, d)
	if err != nil || requests.Load() != 1 || delay < 90 {
		t.Fatalf("delay=%d requests=%d err=%v", delay, requests.Load(), err)
	}
}

func TestUnifiedDelayRedialsThroughOutbound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	d := &latencyTestDialer{address: server.Listener.Addr().String()}
	_, err := URLTest(WithUnifiedDelay(context.Background()), "http://landing.invalid/probe", d)
	if err != nil || d.dials.Load() != 2 {
		t.Fatalf("dials=%d err=%v", d.dials.Load(), err)
	}
}

func TestUnifiedDelayDoesNotBypassFailedEntry(t *testing.T) {
	d := &latencyTestDialer{fail: true}
	_, err := URLTest(WithUnifiedDelay(context.Background()), "http://landing.invalid/probe", d)
	if err == nil || d.dials.Load() != 1 {
		t.Fatalf("dials=%d err=%v", d.dials.Load(), err)
	}
}

func TestUnifiedDelayCancellation(t *testing.T) {
	var requests atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 2 {
			cancel()
			<-r.Context().Done()
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	d := &latencyTestDialer{address: server.Listener.Addr().String()}
	_, err := URLTest(WithUnifiedDelay(ctx), server.URL, d)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestUnifiedDelayWarmFailureUsesColdSample(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) > 1 {
			time.Sleep(150 * time.Millisecond)
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	d := &latencyTestDialer{address: server.Listener.Addr().String()}
	delay, err := URLTest(WithUnifiedDelay(context.Background()), server.URL, d)
	if err != nil || delay >= 150 {
		t.Fatalf("delay=%d err=%v", delay, err)
	}
}

func TestLatencyMillisecondsBounds(t *testing.T) {
	if latencyMilliseconds(0) != 1 || latencyMilliseconds(70*time.Second) != 65535 {
		t.Fatal("invalid bounds")
	}
}
