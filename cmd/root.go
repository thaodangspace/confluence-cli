package cmd

import (
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is the confluence-cli release version. Release builds set it via
// -ldflags "-X github.com/dtonair/confluence-cli/cmd.version=...". For
// `go install`-ed builds it falls back to the module version from build info.
var version = "dev"

// resolveVersion prefers the ldflag-injected version and otherwise reports the
// module version recorded by `go install` (e.g. v0.1.0).
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

// Persistent flags shared by all subcommands.
var (
	flagSpace  string
	flagPretty bool
)

var rootCmd = &cobra.Command{
	Use:           "confluence-cli",
	Short:         "Confluence Cloud CLI for agents and humans",
	Long:          "confluence-cli exposes Confluence Cloud spaces, pages, and comments as\nscriptable commands over the REST v2 API. Output is JSON by default; pass\n--pretty for human-readable text. Credentials come from environment variables\nor a YAML config file.",
	Version:       resolveVersion(),
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command and exits non-zero on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagSpace, "space", "", "Space key or numeric id (defaults to CONFLUENCE_DEFAULT_SPACE)")
	pf.BoolVar(&flagPretty, "pretty", false, "Render human-readable text instead of JSON")
}
