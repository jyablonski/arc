package cmd

import (
	"fmt"
	"os"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/output"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate that all required tools are available",
	Long: `Check if all required and optional tools are available in PATH.
Required tools are necessary for basic functionality, while optional tools
enable additional features.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		style := output.StyleFor(os.Stdout)
		sc := &output.Screen{W: os.Stdout, Style: style, Title: "arc validate", Meta: app.Platform.String()}
		grid := output.Grid{Columns: []output.Column{
			{Align: output.AlignCenter},
			{Header: "tool"},
			{Header: "kind"},
			{Header: "description", Flex: true},
		}}
		// Required tools lead; a missing optional tool is informational only.
		missingRequired, missingOptional := 0, 0
		for _, required := range []bool{true, false} {
			for _, tool := range app.Tools {
				if tool.Required != required {
					continue
				}
				kind, glyph := "optional", output.GlyphOK
				if required {
					kind = "required"
				}
				if !run.CommandExists(tool.Name) {
					if required {
						glyph = output.GlyphFail
						missingRequired++
					} else {
						glyph = output.GlyphInfo
						missingOptional++
					}
				}
				grid.Rows = append(grid.Rows, []string{style.Glyph(glyph), tool.Name, kind, tool.Description})
			}
		}
		sc.Grid(grid)

		if missingRequired > 0 {
			sc.Flush(style.Glyph(output.GlyphFail) + " " + output.Count(missingRequired, "required tool", "required tools") + " missing" + style.Sep() + style.Faint("arc setup"))
			return arcerrs.ErrValidationFailed
		}
		summary := style.Glyph(output.GlyphOK) + " all required tools available"
		if missingOptional > 0 {
			summary += style.Sep() + fmt.Sprintf("%d optional not installed", missingOptional)
		}
		sc.Flush(summary)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
