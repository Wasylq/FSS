// Package cobrai18n walks an assembled cobra command tree and rewrites its
// help strings (Short, Long, flag Usage, and the usage-template chrome)
// through internal/i18n's active catalog.
package cobrai18n

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/Anastylosis/FSS/internal/i18n"
)

const (
	keyUsage      = "Usage:"
	keyAliases    = "Aliases:"
	keyExamples   = "Examples:"
	keyAvailable  = "Available Commands:"
	keyAdditional = "Additional Commands:"
	keyHelpTopics = "Additional help topics:"
	keyFlags      = "Flags:"
	keyGlobal     = "Global Flags:"
	keyUseForMore = `Use "%s [command] --help" for more information about a command.`
	keyErrPrefix  = "Error:"

	keyHelpFor        = "help for %s"
	keyHelpForThis    = "help for this command"
	keyVersionFor     = "version for %s"
	keyVersionForThis = "version for this command"
	keyHelpShort      = "Help about any command"
	keyHelpLong       = "Help provides help for any command in the application.\nSimply type %s help [path to command] for full details."
)

// translateUseLine stays false. Name() is Use up to the first space and
// Find() matches against it, so any edit is one bug from renaming the
// command; UseLine() does strings.Replace(c.Use, c.Name(), …) and garbles if
// they diverge; and <studio-url>/[command] are shell-level metavariables
// every major CLI leaves in ASCII.
const translateUseLine = false

// visit calls fn on root and every descendant.
func visit(c *cobra.Command, fn func(*cobra.Command)) {
	fn(c)
	for _, sub := range c.Commands() {
		visit(sub, fn)
	}
}

// prepare forces the lazy construction cobra normally defers until the
// command being run needs it, so the whole tree has help/version chrome to
// translate up front. Not InitDefaultCompletionCmd: fss ships its own
// cmd/completion.go, so cobra's own completion command is never reached. If
// fss ever drops it, cobra's replacement won't be localized.
func prepare(root *cobra.Command) {
	root.InitDefaultVersionFlag()
	root.InitDefaultHelpCmd()
	visit(root, func(c *cobra.Command) {
		c.InitDefaultHelpFlag()
	})
}

var chromeKeys = []string{
	keyUsage, keyAliases, keyExamples, keyAvailable, keyAdditional,
	keyHelpTopics, keyFlags, keyGlobal, keyUseForMore, keyErrPrefix,
	keyHelpFor, keyHelpForThis, keyVersionFor, keyVersionForThis,
	keyHelpShort, keyHelpLong,
}

// walkFlags visits every flag of c (local and persistent) exactly once
// across the whole tree walk, via pointer identity in seen.
func walkFlags(c *cobra.Command, seen map[*pflag.Flag]bool, fn func(*pflag.Flag)) {
	visitOne := func(f *pflag.Flag) {
		if seen[f] {
			return
		}
		seen[f] = true
		fn(f)
	}
	c.Flags().VisitAll(visitOne)
	c.PersistentFlags().VisitAll(visitOne)
}

func isCobraAnnotated(f *pflag.Flag) bool {
	return len(f.Annotations[cobra.FlagSetByCobraAnnotation]) > 0
}

// Keys returns every translatable string the tree exposes, including the
// chrome and parameterized keys. Same walk as Localize, so the extractor
// cannot drift from the localizer.
func Keys(root *cobra.Command) []string {
	prepare(root)

	keys := append([]string{}, chromeKeys...)
	seen := map[*pflag.Flag]bool{}

	visit(root, func(c *cobra.Command) {
		if c.Name() != "help" {
			if c.Short != "" {
				keys = append(keys, c.Short)
			}
			if c.Long != "" {
				keys = append(keys, c.Long)
			}
		}
		walkFlags(c, seen, func(f *pflag.Flag) {
			if isCobraAnnotated(f) {
				return
			}
			keys = append(keys, f.Usage)
		})
	})

	return keys
}

// Localize rewrites the tree's help strings through the active catalog and
// returns a function that restores every string it changed. Production
// (cmd.Execute) discards the restore func; tests defer it so the shared
// rootCmd singleton is returned to its English state for the next test.
func Localize(root *cobra.Command) (restore func()) {
	// No-op for English: default output stays byte-identical to today — no
	// template substitution, no flag mutation, no help command materialised
	// earlier than cobra would have done it.
	if i18n.Language() == i18n.SourceLanguage {
		return func() {}
	}

	prepare(root)

	var undo []func()
	seen := map[*pflag.Flag]bool{}

	translateFlag := func(c *cobra.Command, f *pflag.Flag) {
		old := f.Usage
		if isCobraAnnotated(f) {
			name := c.DisplayName()
			switch f.Name {
			case "help":
				if name == "" {
					f.Usage = i18n.T(keyHelpForThis)
				} else {
					f.Usage = fmt.Sprintf(i18n.T(keyHelpFor), name)
				}
			case "version":
				if name == "" {
					f.Usage = i18n.T(keyVersionForThis)
				} else {
					f.Usage = fmt.Sprintf(i18n.T(keyVersionFor), name)
				}
			}
		} else {
			f.Usage = i18n.T(f.Usage)
		}
		undo = append(undo, func() { f.Usage = old })
	}

	visit(root, func(c *cobra.Command) {
		if c.Name() == "help" {
			oldShort, oldLong := c.Short, c.Long
			c.Short = i18n.T(keyHelpShort)
			c.Long = fmt.Sprintf(i18n.T(keyHelpLong), root.DisplayName())
			undo = append(undo, func() { c.Short, c.Long = oldShort, oldLong })
		} else {
			if c.Short != "" {
				old := c.Short
				c.Short = i18n.T(c.Short)
				undo = append(undo, func() { c.Short = old })
			}
			if c.Long != "" {
				old := c.Long
				c.Long = i18n.T(c.Long)
				undo = append(undo, func() { c.Long = old })
			}
		}

		walkFlags(c, seen, func(f *pflag.Flag) { translateFlag(c, f) })
	})

	root.SetUsageTemplate(usageTemplate())
	root.SetErrPrefix(i18n.T(keyErrPrefix))

	return func() {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
		root.SetUsageTemplate("")
		root.SetErrPrefix("")
	}
}

// usageTemplate copies cobra's defaultUsageTemplate structure byte-for-byte,
// routing each of its 9 labels through l(). With English active this must
// equal new(cobra.Command).UsageTemplate() byte-for-byte.
func usageTemplate() string {
	l := func(s string) string { return safe(i18n.T(s)) }
	more := strings.Replace(l(keyUseForMore), "%s", "{{.CommandPath}}", 1)

	return l(keyUsage) + `{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

` + l(keyAliases) + `
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

` + l(keyExamples) + `
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

` + l(keyAvailable) + `{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

` + l(keyAdditional) + `{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

` + l(keyFlags) + `
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

` + l(keyGlobal) + `
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

` + l(keyHelpTopics) + `{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

` + more + `{{end}}
`
}

// safe strips {{ and }} before splicing translated text into template
// source. cobra parses lazily inside the render closure with
// template.Must, so a stray delimiter would panic the process on
// `fss --help` rather than failing at startup.
func safe(s string) string {
	s = strings.ReplaceAll(s, "{{", "")
	return strings.ReplaceAll(s, "}}", "")
}
