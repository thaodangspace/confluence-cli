package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/dtonair/confluence-cli/confluence"
	"github.com/dtonair/confluence-cli/output"

	"github.com/spf13/cobra"
)

func init() {
	commentCmd := &cobra.Command{
		Use:   "comment",
		Short: "Comment commands",
	}

	var (
		crPage    string
		crBody    string
		crBodyF   string
		crFormat  string
		crReplyTo string
	)
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Post a footer comment on a page",
		Long: "Post a footer comment on a Confluence page, or a threaded reply to an " +
			"existing comment. This is a write operation; use only when the user has " +
			"asked to post the comment. The body is sent verbatim in the chosen " +
			"representation (no markdown conversion).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pageID, err := parseID("page id", crPage)
			if err != nil {
				return fail(err)
			}
			if !validWriteBodyFormat(crFormat) {
				return fail(fmt.Errorf("invalid --body-format %q: use storage or atlas_doc_format", crFormat))
			}
			body, bodyProvided, err := readBody(crBody, crBodyF, os.Stdin)
			if err != nil {
				return fail(err)
			}
			if !bodyProvided || strings.TrimSpace(body) == "" {
				return fail(fmt.Errorf("Provide a non-empty --body or --body-file."))
			}

			var replyTo string
			if strings.TrimSpace(crReplyTo) != "" {
				replyTo, err = parseID("reply-to comment id", crReplyTo)
				if err != nil {
					return fail(err)
				}
			}

			_, client, err := newClient()
			if err != nil {
				return fail(err)
			}

			payload := map[string]any{
				"pageId": pageID,
				"body": map[string]any{
					"representation": crFormat,
					"value":          body,
				},
			}
			if replyTo != "" {
				payload["parentCommentId"] = replyTo
			}

			var raw json.RawMessage
			if err := client.Request(ctx(cmd), "/footer-comments", confluence.RequestOptions{Method: http.MethodPost, Body: payload}, &raw); err != nil {
				return fail(err)
			}
			if err := emitObject(raw, output.CommentSummary); err != nil {
				return fail(err)
			}
			return nil
		},
	}
	createCmd.Flags().StringVar(&crPage, "page", "", "Page id to comment on (required)")
	createCmd.Flags().StringVar(&crBody, "body", "", "Comment body in the chosen representation")
	createCmd.Flags().StringVar(&crBodyF, "body-file", "", "Read comment body from file (- for stdin)")
	createCmd.Flags().StringVar(&crFormat, "body-format", "storage", "Body representation: storage or atlas_doc_format")
	createCmd.Flags().StringVar(&crReplyTo, "reply-to", "", "Parent comment id to reply to (optional)")
	_ = createCmd.MarkFlagRequired("page")

	commentCmd.AddCommand(createCmd)
	rootCmd.AddCommand(commentCmd)
}
