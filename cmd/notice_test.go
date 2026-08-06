package cmd

import (
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/config"
)

// resetNotice clears the once-per-process guard so each case starts fresh.
func resetNotice(t *testing.T) {
	t.Helper()
	noticeOnce = onceReset()
	t.Cleanup(func() { noticeOnce = onceReset() })
}

func TestFlatStoreNoticeShownOnceOnFlatStore(t *testing.T) {
	withCfg(t, &config.Config{})
	resetNotice(t)
	t.Setenv("FSS_NO_NOTICES", "")

	out := captureStderr(t, func() { warnFlatStoreDefaultChanging(false) })
	if !strings.Contains(out, "SQLite store by default") {
		t.Errorf("notice not printed: %q", out)
	}
	if !strings.Contains(out, "Nothing changes today") {
		t.Errorf("notice does not reassure that nothing changes yet: %q", out)
	}
	if !strings.Contains(out, "fss import") {
		t.Errorf("notice does not say how to move now: %q", out)
	}

	// Repeat calls stay quiet, so scraping several URLs prints it once.
	again := captureStderr(t, func() { warnFlatStoreDefaultChanging(false) })
	if again != "" {
		t.Errorf("notice repeated within one process: %q", again)
	}
}

// Someone already using --db does not need to be told about a change that
// does not affect them.
func TestFlatStoreNoticeSilentWhenUsingDB(t *testing.T) {
	withCfg(t, &config.Config{})
	resetNotice(t)
	t.Setenv("FSS_NO_NOTICES", "")

	if out := captureStderr(t, func() { warnFlatStoreDefaultChanging(true) }); out != "" {
		t.Errorf("notice shown to a database user: %q", out)
	}
}

func TestFlatStoreNoticeSuppressible(t *testing.T) {
	t.Run("env var", func(t *testing.T) {
		withCfg(t, &config.Config{})
		resetNotice(t)
		t.Setenv("FSS_NO_NOTICES", "1")
		if out := captureStderr(t, func() { warnFlatStoreDefaultChanging(false) }); out != "" {
			t.Errorf("FSS_NO_NOTICES did not silence the notice: %q", out)
		}
	})

	t.Run("config notices: false", func(t *testing.T) {
		off := false
		withCfg(t, &config.Config{Notices: &off})
		resetNotice(t)
		t.Setenv("FSS_NO_NOTICES", "")
		if out := captureStderr(t, func() { warnFlatStoreDefaultChanging(false) }); out != "" {
			t.Errorf("`notices: false` did not silence the notice: %q", out)
		}
	})

	// An absent `notices` key means enabled — the zero value must not silence it.
	t.Run("absent config key leaves it on", func(t *testing.T) {
		withCfg(t, &config.Config{})
		resetNotice(t)
		t.Setenv("FSS_NO_NOTICES", "")
		if out := captureStderr(t, func() { warnFlatStoreDefaultChanging(false) }); out == "" {
			t.Error("notice suppressed when `notices` was simply absent")
		}
	})
}
