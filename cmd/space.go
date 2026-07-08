package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/dtonair/confluence-cli/confluence"
	"github.com/dtonair/confluence-cli/output"

	"github.com/spf13/cobra"
)

func init() {
	spaceCmd := &cobra.Command{
		Use:   "space",
		Short: "Space commands",
	}

	var (
		listKeys   string
		listType   string
		listStatus string
		listLimit  int
	)
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List spaces",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, client, err := newClient()
			if err != nil {
				return fail(err)
			}

			q := url.Values{"limit": {fmt.Sprint(confluence.DefaultPageLen)}}
			if s := strings.TrimSpace(listKeys); s != "" {
				q.Set("keys", s)
			}
			if listType != "" {
				q.Set("type", listType)
			}
			if listStatus != "" {
				q.Set("status", listStatus)
			}
			path := "/spaces?" + q.Encode()

			values, err := client.Paginate(ctx(cmd), path, listLimit, confluence.DefaultMaxPages)
			if err != nil {
				return fail(err)
			}
			if err := emitList(values, output.SpaceSummary, "No spaces found."); err != nil {
				return fail(err)
			}
			return nil
		},
	}
	listCmd.Flags().StringVar(&listKeys, "keys", "", "Comma-separated space keys to filter by")
	listCmd.Flags().StringVar(&listType, "type", "", "Filter by type: global or personal")
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status: current or archived")
	listCmd.Flags().IntVar(&listLimit, "limit", confluence.DefaultLimit, "Maximum spaces to return")

	getCmd := &cobra.Command{
		Use:   "get <id|key>",
		Short: "Get a single space by numeric id or key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, err := newClient()
			if err != nil {
				return fail(err)
			}
			id, err := spaceIDFor(cmd, client, args[0])
			if err != nil {
				return fail(err)
			}

			var raw json.RawMessage
			path := fmt.Sprintf("/spaces/%s", confluence.EncodePathSegment(id))
			if err := client.Request(ctx(cmd), path, confluence.RequestOptions{}, &raw); err != nil {
				return fail(err)
			}
			if err := emitObject(raw, output.SpaceSummary); err != nil {
				return fail(err)
			}
			return nil
		},
	}

	spaceCmd.AddCommand(listCmd, getCmd)
	rootCmd.AddCommand(spaceCmd)
}
