//go:build windows

package main

import (
	"context"
	"errors"
	"testing"
)

func TestCancelledSystemProxyWriteAfterLockDoesNotChangePreference(t *testing.T) {
	p := &windowsPlatformInterface{systemProxyEnabled: true}
	p.access.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- p.SetSystemProxyEnabledContext(ctx, false) }()
	cancel()
	p.access.Unlock()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if !p.systemProxyEnabled {
		t.Fatal("cancelled request changed the proxy preference")
	}
}
