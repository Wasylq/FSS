package scraper

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// captureStderr redirects os.Stderr to a pipe for the duration of fn,
// returning everything written to stderr. Restores the original Stderr
// on exit.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w

	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 0, 1024)
		tmp := make([]byte, 256)
		for {
			n, err := r.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
			}
			if err != nil {
				break
			}
		}
		done <- string(buf)
	}()

	fn()
	_ = w.Close()
	os.Stderr = orig
	return <-done
}

func TestWarnDelayBelow_emits(t *testing.T) {
	ResetWarnDelayBelow()
	out := captureStderr(t, func() {
		WarnDelayBelow("julesjordan", 100*time.Millisecond, 500*time.Millisecond)
	})
	if !strings.Contains(out, "julesjordan") {
		t.Errorf("warning should mention site ID: %q", out)
	}
	if !strings.Contains(out, "100ms") || !strings.Contains(out, "500ms") {
		t.Errorf("warning should include both values: %q", out)
	}
	if !strings.Contains(out, "site_delays") || !strings.Contains(out, "--site-delay") {
		t.Errorf("warning should suggest both config and CLI overrides: %q", out)
	}
}

func TestWarnDelayBelow_onceOnly(t *testing.T) {
	ResetWarnDelayBelow()
	out := captureStderr(t, func() {
		WarnDelayBelow("siteX", 0, 500*time.Millisecond)
		WarnDelayBelow("siteX", 50*time.Millisecond, 500*time.Millisecond)
		WarnDelayBelow("siteX", 100*time.Millisecond, 500*time.Millisecond)
	})
	if c := strings.Count(out, "[warn]"); c != 1 {
		t.Errorf("expected exactly 1 warning, got %d: %q", c, out)
	}
}

func TestWarnDelayBelow_perSite(t *testing.T) {
	ResetWarnDelayBelow()
	out := captureStderr(t, func() {
		WarnDelayBelow("siteA", 0, 500*time.Millisecond)
		WarnDelayBelow("siteB", 0, 500*time.Millisecond)
	})
	if c := strings.Count(out, "[warn]"); c != 2 {
		t.Errorf("expected 2 warnings (one per site), got %d: %q", c, out)
	}
}

func TestWarnDelayBelow_noopWhenSatisfied(t *testing.T) {
	ResetWarnDelayBelow()
	out := captureStderr(t, func() {
		WarnDelayBelow("siteX", 500*time.Millisecond, 500*time.Millisecond)
		WarnDelayBelow("siteY", 1*time.Second, 500*time.Millisecond)
	})
	if out != "" {
		t.Errorf("no warning expected, got %q", out)
	}
}

func TestWarnDelayBelow_noopWhenRecommendedZero(t *testing.T) {
	ResetWarnDelayBelow()
	out := captureStderr(t, func() {
		WarnDelayBelow("siteX", 0, 0)
		WarnDelayBelow("siteX", 1*time.Millisecond, 0)
	})
	if out != "" {
		t.Errorf("no warning expected when recommended<=0, got %q", out)
	}
}

func TestWarnDelayBelow_concurrent(t *testing.T) {
	ResetWarnDelayBelow()
	var wg sync.WaitGroup
	out := captureStderr(t, func() {
		for i := 0; i < 32; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				WarnDelayBelow("siteX", 0, 500*time.Millisecond)
			}()
		}
		wg.Wait()
	})
	if c := strings.Count(out, "[warn]"); c != 1 {
		t.Errorf("expected exactly 1 warning under concurrent calls, got %d: %q", c, out)
	}
}
