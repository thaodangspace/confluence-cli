package cmd

import (
	"fmt"

	"github.com/dtonair/confluence-cli/confluence"
	"github.com/dtonair/confluence-cli/output"

	"github.com/spf13/cobra"
)

func init() {
	attachmentCmd := &cobra.Command{
		Use:   "attachment",
		Short: "Attachment commands",
	}

	var listLimit int
	listCmd := &cobra.Command{
		Use:   "list <page-id>",
		Short: "List attachments on a page",
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

			path := fmt.Sprintf("/pages/%s/attachments?limit=%d",
				confluence.EncodePathSegment(id), confluence.DefaultPageLen)
			values, err := client.Paginate(ctx(cmd), path, listLimit, confluence.DefaultMaxPages)
			if err != nil {
				return fail(err)
			}
			if err := emitList(values, output.AttachmentSummary, "No attachments found."); err != nil {
				return fail(err)
			}
			return nil
		},
	}
	listCmd.Flags().IntVar(&listLimit, "limit", confluence.DefaultLimit, "Maximum attachments to return")

	attachmentCmd.AddCommand(listCmd)
	rootCmd.AddCommand(attachmentCmd)
}
