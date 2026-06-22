//go:build integration

package finishesthejob

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveManojob(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("manojob"), "https://www.manojob.com/", 3)
}

func TestLiveMrpov(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("mrpov"), "https://www.mrpov.com/", 3)
}

func TestLiveTheDickSuckers(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("thedicksuckers"), "https://www.thedicksuckers.com/", 3)
}

func TestLiveFinishesTheJob(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("finishesthejob"), "https://www.finishesthejob.com/", 3)
}
