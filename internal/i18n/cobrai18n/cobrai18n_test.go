package cobrai18n_test

import (
	"sort"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Anastylosis/FSS/internal/i18n"
	"github.com/Anastylosis/FSS/internal/i18n/cobrai18n"
)

func TestUsageTemplateMatchesCobra(t *testing.T) {
	i18n.SetLanguage(i18n.SourceLanguage)
	t.Cleanup(func() { i18n.SetLanguage(i18n.SourceLanguage) })

	got := cobrai18n.UsageTemplateForTest()
	want := new(cobra.Command).UsageTemplate()
	if got != want {
		t.Errorf("usageTemplate() does not match cobra's default:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCobraDefaultsUnchanged(t *testing.T) {
	t.Run("help flag", func(t *testing.T) {
		c := &cobra.Command{Use: "x"}
		c.InitDefaultHelpFlag()
		f := c.Flags().Lookup("help")
		if f == nil || f.Usage != "help for x" {
			t.Fatalf("help flag usage = %v, want %q", f, "help for x")
		}
	})

	t.Run("version flag", func(t *testing.T) {
		c := &cobra.Command{Use: "x", Version: "1.0"}
		c.InitDefaultVersionFlag()
		f := c.Flags().Lookup("version")
		if f == nil || f.Usage != "version for x" {
			t.Fatalf("version flag usage = %v, want %q", f, "version for x")
		}
	})

	t.Run("help command", func(t *testing.T) {
		parent := &cobra.Command{Use: "root"}
		child := &cobra.Command{Use: "child", Run: func(*cobra.Command, []string) {}}
		parent.AddCommand(child)
		parent.InitDefaultHelpCmd()

		var help *cobra.Command
		for _, c := range parent.Commands() {
			if c.Name() == "help" {
				help = c
			}
		}
		if help == nil {
			t.Fatal("no help command added")
		}
		if help.Short != "Help about any command" {
			t.Errorf("help.Short = %q, want %q", help.Short, "Help about any command")
		}
		wantLong := "Help provides help for any command in the application.\nSimply type root help [path to command] for full details."
		if help.Long != wantLong {
			t.Errorf("help.Long = %q, want %q", help.Long, wantLong)
		}
	})
}

func TestSafeStripsDelimiters(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"double delimiters", "{{Usage}}", "Usage"},
		{"nested delimiters", "{{{{x}}}}", "x"},
		{"no delimiters", "plain text", "plain text"},
		{"only opens", "{{{{", ""},
		{"unrelated braces", `Use "%s [command] --help"`, `Use "%s [command] --help"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cobrai18n.SafeForTest(tt.in); got != tt.want {
				t.Errorf("safe(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func buildSyntheticTree() (root, child *cobra.Command) {
	root = &cobra.Command{
		Use:   "root",
		Short: "root short",
		Long:  "root long",
	}
	child = &cobra.Command{
		Use:   "child",
		Short: "child short",
		Long:  "child long",
		Run:   func(*cobra.Command, []string) {},
	}
	root.AddCommand(child)
	root.PersistentFlags().String("persist", "", "persist usage")
	child.Flags().String("local", "", "local usage")
	return root, child
}

func TestLocalizeSyntheticTree(t *testing.T) {
	t.Run("english is a no-op", func(t *testing.T) {
		i18n.SetLanguage(i18n.SourceLanguage)
		t.Cleanup(func() { i18n.SetLanguage(i18n.SourceLanguage) })

		root, child := buildSyntheticTree()
		wantTemplate := root.UsageTemplate()
		rootShort, rootLong := root.Short, root.Long
		childShort, childLong := child.Short, child.Long
		persistUsage := root.PersistentFlags().Lookup("persist").Usage
		localUsage := child.Flags().Lookup("local").Usage

		restore := cobrai18n.Localize(root)
		restore()

		if root.Short != rootShort || root.Long != rootLong {
			t.Errorf("root Short/Long changed: got (%q, %q)", root.Short, root.Long)
		}
		if child.Short != childShort || child.Long != childLong {
			t.Errorf("child Short/Long changed: got (%q, %q)", child.Short, child.Long)
		}
		if got := root.PersistentFlags().Lookup("persist").Usage; got != persistUsage {
			t.Errorf("persist flag usage changed: got %q", got)
		}
		if got := child.Flags().Lookup("local").Usage; got != localUsage {
			t.Errorf("local flag usage changed: got %q", got)
		}
		if root.UsageTemplate() != wantTemplate {
			t.Errorf("UsageTemplate() changed for a no-op Localize")
		}
	})

	t.Run("Keys returns the expected set", func(t *testing.T) {
		root, _ := buildSyntheticTree()

		want := map[string]bool{
			"root short": true, "root long": true,
			"child short": true, "child long": true,
			"persist usage": true, "local usage": true,
		}
		for _, k := range []string{
			"Usage:", "Aliases:", "Examples:", "Available Commands:", "Additional Commands:",
			"Additional help topics:", "Flags:", "Global Flags:",
			`Use "%s [command] --help" for more information about a command.`, "Error:",
			"help for %s", "help for this command", "version for %s", "version for this command",
			"Help about any command",
			"Help provides help for any command in the application.\nSimply type %s help [path to command] for full details.",
		} {
			want[k] = true
		}

		got := cobrai18n.Keys(root)
		gotSet := map[string]bool{}
		for _, k := range got {
			gotSet[k] = true
		}

		if len(gotSet) != len(want) {
			var gotList, wantList []string
			for k := range gotSet {
				gotList = append(gotList, k)
			}
			for k := range want {
				wantList = append(wantList, k)
			}
			sort.Strings(gotList)
			sort.Strings(wantList)
			t.Fatalf("Keys() set mismatch:\ngot:  %v\nwant: %v", gotList, wantList)
		}
		for k := range want {
			if !gotSet[k] {
				t.Errorf("Keys() missing %q", k)
			}
		}
		for k := range gotSet {
			if !want[k] {
				t.Errorf("Keys() has unexpected key %q", k)
			}
		}
	})
}
