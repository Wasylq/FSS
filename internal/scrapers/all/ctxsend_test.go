package all

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CLAUDE.md requires every send to a scraper's output channel to sit in a select
// that also selects on ctx.Done(). Nothing verified it. Deleting the guard breaks
// no test — the scrape still works — and the cost is a goroutine that keeps
// fetching pages nobody will read, one per cancelled scrape.
//
// testutil.AssertCancellable covers this at runtime but needs a per-scraper test
// server, so it can only ever be added a package at a time; ~350 scrapers would
// each need a bespoke test. This checks the same rule statically across every
// send site in every scraper package at once.
//
// The check runs on source rather than behaviour, so it cannot prove a scraper
// *reacts* correctly to cancellation — only that the guard is present. That is
// precisely the regression worth catching: the guard is easy to omit when adding
// a send, and its absence is invisible.
func TestScraperSendsAreGuardedByContext(t *testing.T) {
	root := filepath.Join("..")
	var sends, guarded int
	type site struct {
		pkg  string
		file string
		line int
	}
	var unguarded []site

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Errorf("parsing %s: %v", path, perr)
			return nil
		}

		// A send is guarded when it is one case of a select that has another case
		// receiving from something ending in Done() — ctx.Done() in practice, but
		// matching on the call keeps this working for a renamed context variable.
		guardedSends := map[ast.Node]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectStmt)
			if !ok {
				return true
			}
			hasDone := false
			for _, c := range sel.Body.List {
				cc, ok := c.(*ast.CommClause)
				if !ok || cc.Comm == nil {
					continue
				}
				if strings.Contains(render(fset, cc.Comm), "Done()") {
					hasDone = true
					break
				}
			}
			if !hasDone {
				return true
			}
			for _, c := range sel.Body.List {
				cc, ok := c.(*ast.CommClause)
				if !ok || cc.Comm == nil {
					continue
				}
				if s, ok := cc.Comm.(*ast.SendStmt); ok {
					guardedSends[s] = true
				}
			}
			return true
		})

		ast.Inspect(f, func(n ast.Node) bool {
			s, ok := n.(*ast.SendStmt)
			if !ok {
				return true
			}
			if !isSceneResultSend(render(fset, s.Value)) {
				return true
			}
			sends++
			if guardedSends[s] {
				guarded++
				return true
			}
			unguarded = append(unguarded, site{
				pkg:  filepath.Base(filepath.Dir(path)),
				file: filepath.Base(path),
				line: fset.Position(s.Pos()).Line,
			})
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	for _, u := range unguarded {
		t.Errorf("%s/%s:%d: send to the output channel is not inside a select with "+
			"ctx.Done() — on cancellation this blocks forever and leaks the scraper "+
			"goroutine (CLAUDE.md: \"every channel send must be in a select with "+
			"case <-ctx.Done()\")", u.pkg, u.file, u.line)
	}

	// Reported so a drop in coverage is visible rather than looking like a pass:
	// if a refactor moves sends behind a helper this test cannot see, the count
	// falls and that is worth noticing.
	t.Logf("%d output-channel sends checked, %d guarded", sends, guarded)
	if sends == 0 {
		t.Fatal("no sends found — the detector is broken, not the scrapers " +
			"(scrapers using scraper.Paginate have no direct sends, but not all of them do)")
	}
}

// isSceneResultSend reports whether a sent value is one of the SceneResult
// constructors. Matching the constructors rather than the channel's type keeps
// this independent of what each scraper names its channel, and excludes the
// internal work-queue sends (`work <- ls`), which are a different concern —
// those close on their own when the producer returns.
func isSceneResultSend(v string) bool {
	for _, ctor := range []string{
		"scraper.Scene(", "scraper.Error(", "scraper.Progress(", "scraper.StoppedEarly(",
	} {
		if strings.Contains(v, ctor) {
			return true
		}
	}
	return false
}

func render(fset *token.FileSet, n ast.Node) string {
	var sb strings.Builder
	if err := format.Node(&sb, fset, n); err != nil {
		return ""
	}
	return sb.String()
}
