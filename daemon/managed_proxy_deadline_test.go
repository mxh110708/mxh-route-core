package daemon

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type contextualProxyHandler struct {
	ManagedHandler
	called     bool
	gotContext context.Context
}

func (h *contextualProxyHandler) SetSystemProxyEnabledContext(ctx context.Context, enabled bool) error {
	h.called = enabled
	h.gotContext = ctx
	return nil
}

func TestSystemProxyRPCForwardsContext(t *testing.T) {
	h := &contextualProxyHandler{}
	s := NewManagedService(ManagedServiceOptions{Handler: h})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := s.SetSystemProxyEnabled(ctx, &SetSystemProxyEnabledRequest{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if !h.called || h.gotContext != ctx {
		t.Fatal("write did not receive RPC context")
	}
}

func TestSystemProxyRPCRejectsExpiredRequest(t *testing.T) {
	h := &contextualProxyHandler{}
	s := NewManagedService(ManagedServiceOptions{Handler: h})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.SetSystemProxyEnabled(ctx, &SetSystemProxyEnabledRequest{Enabled: true}); status.Code(err) != codes.Canceled {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if h.called {
		t.Fatal("cancelled request reached handler")
	}
}
