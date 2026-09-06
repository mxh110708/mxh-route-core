package main

import "testing"

func TestDesktopUpdateVersion(t *testing.T) {
	for _, c := range []struct {
		candidate, installed string
		newer                bool
	}{
		{"1.14.0-mxh.2", "1.14.0-mxh.1", true},
		{"1.14.0-mxh.10", "1.14.0-mxh.9", true},
		{"1.14.0-beta.14.mxh.8", "1.14.0-beta.14.mxh.7", true},
		{"1.14.0-mxh.1", "1.14.0-beta.14.mxh.7", true},
		{"1.14.0-mxh.2", "1.14.0-mxh.2", false},
		{"1.14.0-mxh.1", "1.14.0-mxh.2", false},
		{"invalid", "1.14.0-mxh.1", false},
		{"1.14.0-mxh.2", "invalid", false},
	} {
		if got := isNewerDesktopVersion(c.candidate, c.installed); got != c.newer {
			t.Errorf("%s vs %s: got %v", c.candidate, c.installed, got)
		}
	}
}
