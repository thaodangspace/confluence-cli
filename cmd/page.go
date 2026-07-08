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

// validBodyFormat reports whether fmt is an accepted read representation.
func validReadBodyFormat(f string) bool {
	switch f {
	case "storage", "atlas_doc_format", "view":
		return true
	default:
		return false
	}
}

// pageCmd is the parent for page subcommands. Write subcommands (create,
// update) register themselves onto it from page_write.go.
var pageCmd = &cobra.Command{
	Use:   "page",
	Short: "Page commands",
}

func init() {
	// page list
	var (
		listTitle  string
		listStatus string
		listLimit  int
	)
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List pages, optionally scoped to a space",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, client, err := newClient()
			if err != nil {
				return fail(err)
			}

			q := url.Values{"limit": {fmt.Sprint(confluence.DefaultPageLen)}}
			if s := strings.TrimSpace(listTitle); s != "" {
				q.Set("title", s)
			}
			if listStatus != "" {
				q.Set("status", listStatus)
			}

			// Scope to a space when --space or a default space is set; otherwise
			// list across all pages.
			var path string
			if strings.TrimSpace(flagSpace) != "" || strings.TrimSpace(cfg.DefaultSpace) != "" {
				spaceID, err := resolveSpaceID(cmd, client, cfg)
				if err != nil {
					return fail(err)
				}
				path = fmt.Sprintf("/spaces/%s/pages?%s", confluence.EncodePathSegment(spaceID), q.Encode())
			} else {
				path = "/pages?" + q.Encode()
			}

			values, err := client.Paginate(ctx(cmd), path, listLimit, confluence.DefaultMaxPages)
			if err != nil {
				return fail(err)
			}
			if err := emitList(values, output.PageSummary, "No pages found."); err != nil {
				return fail(err)
			}
			return nil
		},
	}
	listCmd.Flags().StringVar(&listTitle, "title", "", "Filter by exact page title")
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status: current, draft, or archived")
	listCmd.Flags().IntVar(&listLimit, "limit", confluence.DefaultLimit, "Maximum pages to return")

	// page get
	var getBodyFormat string
	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a single page by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("page id", args[0])
			if err != nil {
				return fail(err)
			}
			if !validReadBodyFormat(getBodyFormat) {
				return fail(fmt.Errorf("invalid --body-format %q: use storage, atlas_doc_format, or view", getBodyFormat))
			}
			_, client, err := newClient()
			if err != nil {
				return fail(err)
			}

			var raw json.RawMessage
			path := fmt.Sprintf("/pages/%s?body-format=%s", confluence.EncodePathSegment(id), url.QueryEscape(getBodyFormat))
			if err := client.Request(ctx(cmd), path, confluence.RequestOptions{}, &raw); err != nil {
				return fail(err)
			}
			if err := emitObject(raw, output.PageSummary); err != nil {
				return fail(err)
			}
			return nil
		},
	}
	getCmd.Flags().StringVar(&getBodyFormat, "body-format", "storage", "Body representation: storage, atlas_doc_format, or view")

	// page comments
	var commentsLimit int
	commentsCmd := &cobra.Command{
		Use:   "comments <id>",
		Short: "List footer comments on a page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("page id", args[0])
			if err != nil {
				return fail(err)
			}
			_, client, err := newClient()
			if err != nil {
				return fail(err)
			}

			path := fmt.Sprintf("/pages/%s/footer-comments?limit=%d&body-format=storage",
				confluence.EncodePathSegment(id), confluence.DefaultPageLen)
			values, err := client.Paginate(ctx(cmd), path, commentsLimit, confluence.DefaultMaxPages)
			if err != nil {
				return fail(err)
			}
			if err := emitList(values, output.CommentSummary, "No comments found."); err != nil {
				return fail(err)
			}
			return nil
		},
	}
	commentsCmd.Flags().IntVar(&commentsLimit, "limit", confluence.DefaultLimit, "Maximum comments to return")

	pageCmd.AddCommand(listCmd, getCmd, commentsCmd)
	rootCmd.AddCommand(pageCmd)
}
