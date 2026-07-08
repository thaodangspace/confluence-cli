package cmd

import (
	"fmt"
	"os"

	"github.com/dtonair/confluence-cli/output"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Check confluence-cli configuration",
		Long:  "Report whether credentials resolve from the environment or config file, which site is targeted, and the default space in effect.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return fail(err)
			}

			if flagPretty {
				space := cfg.DefaultSpace
				if space == "" {
					space = "not configured"
				}
				_, err := fmt.Fprintf(os.Stdout, "Confluence configured. Site: %s. Default space: %s\n", cfg.Site, space)
				return err
			}

			return output.RenderJSON(os.Stdout, map[string]any{
				"configured":   true,
				"email":        cfg.Email,
				"site":         cfg.Site,
				"defaultSpace": nullableString(cfg.DefaultSpace),
			})
		},
	})
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
