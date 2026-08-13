package cmd

import (
	"fmt"
	"strings"

	"github.com/Anastylosis/FSS/models"
	"github.com/spf13/cobra"
)

// sceneOverrides carries operator-supplied identity fields that replace what a
// site published. It exists for the case one performer's catalogue is spread
// over several sites that each credit her differently: scraping all of them
// under one `--performer` files every scene under the same name, instead of
// leaving the store with an alias per site.
//
// The override applies to the scenes a run *scrapes*, not to everything the
// studio has stored. Incremental therefore relabels only what it re-collects;
// `--full` and `--refresh` re-collect the catalogue and so relabel all of it.
// Rewriting untouched stored scenes from a flag would be a much larger and far
// less obvious edit than the flag appears to describe.
type sceneOverrides struct {
	performers []string
	studio     string
}

func (o sceneOverrides) empty() bool { return len(o.performers) == 0 && o.studio == "" }

// apply replaces the overridden fields. Performers are replaced outright rather
// than merged: the point is to drop the site's spelling, and a scene that keeps
// both ends up with one performer under two names.
func (o sceneOverrides) apply(s models.Scene) models.Scene {
	if len(o.performers) > 0 {
		s.Performers = append([]string(nil), o.performers...)
	}
	if o.studio != "" {
		s.Studio = o.studio
	}
	return s
}

// parseOverrides reads --performer/--studio. Both accept comma-separated values
// as well as a repeated flag, since `--performer "A, B"` is the form people
// reach for first and silently storing that as a single two-person name would
// be worse than splitting it.
func parseOverrides(cmd *cobra.Command) (sceneOverrides, error) {
	var o sceneOverrides

	raw, _ := cmd.Flags().GetStringArray("performer")
	for _, entry := range raw {
		for _, name := range strings.Split(entry, ",") {
			if name = strings.TrimSpace(name); name != "" {
				o.performers = append(o.performers, name)
			}
		}
	}
	if cmd.Flags().Changed("performer") && len(o.performers) == 0 {
		return sceneOverrides{}, fmt.Errorf("--performer was given no usable name")
	}

	studio, _ := cmd.Flags().GetString("studio")
	o.studio = strings.TrimSpace(studio)
	if cmd.Flags().Changed("studio") && o.studio == "" {
		return sceneOverrides{}, fmt.Errorf("--studio was given no usable name")
	}

	return o, nil
}

// describe reports what a run will relabel, so the override is visible in the
// output rather than only in the stored result.
func (o sceneOverrides) describe() string {
	var parts []string
	if len(o.performers) > 0 {
		parts = append(parts, "performers → "+strings.Join(o.performers, ", "))
	}
	if o.studio != "" {
		parts = append(parts, "studio → "+o.studio)
	}
	return strings.Join(parts, "; ")
}
