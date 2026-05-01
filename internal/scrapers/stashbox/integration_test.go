//go:build integration

package stashbox

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveStashdbPerformer(t *testing.T) {
	if len(getInstances()) == 0 {
		t.Skip("no stashbox instances configured — add one to config.yaml")
	}
	testutil.RunLiveScrape(t, New(), "https://stashdb.org/performers/7a0f7c42-7c45-4fce-911d-0bfbf293707d", 2)
}

func TestLiveStashdbStudio(t *testing.T) {
	if len(getInstances()) == 0 {
		t.Skip("no stashbox instances configured — add one to config.yaml")
	}
	testutil.RunLiveScrape(t, New(), "https://stashdb.org/studios/eec95cdd-9f58-4fc7-b7d1-e98786453d27", 2)
}
