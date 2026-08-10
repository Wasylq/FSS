//go:build integration

package collegeuniform

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveCollegeUniform(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://college-uniform.com/", 3)
}
