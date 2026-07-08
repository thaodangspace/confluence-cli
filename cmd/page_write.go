package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/dtonair/confluence-cli/confluence"
	"github.com/dtonair/confluence-cli/output"

	"github.com/spf13/cobra"
)

// readBody resolves a text body from a flag value, a file path, or stdin. The
// flag and file sources are mutually exclusive. A file source of "-" reads
// stdin. The returned `provided` is true whenever the caller supplied a value:
// a file always counts as provided (even when empty, enabling an explicit
// clear), while a bare flag counts only when non-empty.
func readBody(text, file string, stdin io.Reader) (value string, provided bool, err error) {
	if text != "" && file != "" {
		return "", false, fmt.Errorf("use only one of --body / --body-file")
	}
	if file != "" {
		if file == "-" {
			b, rerr := io.ReadAll(stdin)
			if rerr != nil {
				return "", false, fmt.Errorf("read body from stdin: %w", rerr)
			}
			return string(b), true, nil
		}
		b, rerr := os.ReadFile(file)
		if rerr != nil {
			return "", false, fmt.Errorf("read body file %q: %w", file, rerr)
		}
		return string(b), true, nil
	}
	return text, text != "", nil
}

// validWriteBodyFormat reports whether f is an accepted write representation
// (view is read-only and rejected).
func validWriteBodyFormat(f string) bool {
	switch f {
	case "storage", "atlas_doc_format":
		return true
	default:
		return false
	}
}

func init() {
	// page create
	var (
		crTitle  string
		crParent string
		crStatus string
		crBody   string
		crBodyF  string
		crFormat string
	)
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new page",
		Long: "Create a new Confluence page in a space. This is a write operation; use " +
			"only when the user has asked to create the page. The body is sent verbatim " +
			"in the chosen representation (no markdown conversion).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(crTitle) == "" {
				return fail(fmt.Errorf("Provide --title."))
			}
			if !validWriteBodyFormat(crFormat) {
				return fail(fmt.Errorf("invalid --body-format %q: use storage or atlas_doc_format", crFormat))
			}
			body, bodyProvided, err := readBody(crBody, crBodyF, os.Stdin)
			if err != nil {
				return fail(err)
			}

			cfg, client, err := newClient()
			if err != nil {
				return fail(err)
			}
			spaceID, err := resolveSpaceID(cmd, client, cfg)
			if err != nil {
				return fail(err)
			}

			status := crStatus
			if status == "" {
				status = "current"
			}
			payload := map[string]any{
				"spaceId": spaceID,
				"status":  status,
				"title":   crTitle,
			}
			if strings.TrimSpace(crParent) != "" {
				payload["parentId"] = crParent
			}
			if bodyProvided {
				payload["body"] = map[string]any{
					"representation": crFormat,
					"value":          body,
				}
			}

			var raw json.RawMessage
			if err := client.Request(ctx(cmd), "/pages", confluence.RequestOptions{Method: http.MethodPost, Body: payload}, &raw); err != nil {
				return fail(err)
			}
			if err := emitObject(raw, output.PageSummary); err != nil {
				return fail(err)
			}
			return nil
		},
	}
	createCmd.Flags().StringVar(&crTitle, "title", "", "Page title (required)")
	createCmd.Flags().StringVar(&crParent, "parent", "", "Parent page id (optional)")
	createCmd.Flags().StringVar(&crStatus, "status", "current", "Page status: current or draft")
	createCmd.Flags().StringVar(&crBody, "body", "", "Page body in the chosen representation")
	createCmd.Flags().StringVar(&crBodyF, "body-file", "", "Read body from file (- for stdin)")
	createCmd.Flags().StringVar(&crFormat, "body-format", "storage", "Body representation: storage or atlas_doc_format")
	_ = createCmd.MarkFlagRequired("title")

	// page update — read-modify-write to bump version.number.
	var (
		upTitle  string
		upStatus string
		upBody   string
		upBodyF  string
		upFormat string
		upMsg    string
	)
	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a page's title, body, and/or status",
		Long: "Update an existing Confluence page. This is a write operation; use only " +
			"when the user has asked to update the page. Confluence requires the version " +
			"number to increment and re-sends title/body/status, so this command reads " +
			"the current page and carries over any field you do not change.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("page id", args[0])
			if err != nil {
				return fail(err)
			}
			if !validWriteBodyFormat(upFormat) {
				return fail(fmt.Errorf("invalid --body-format %q: use storage or atlas_doc_format", upFormat))
			}
			body, bodyProvided, err := readBody(upBody, upBodyF, os.Stdin)
			if err != nil {
				return fail(err)
			}
			titleProvided := strings.TrimSpace(upTitle) != ""
			statusProvided := strings.TrimSpace(upStatus) != ""
			if !titleProvided && !statusProvided && !bodyProvided {
				return fail(fmt.Errorf("Provide at least one of --title, --body/--body-file, or --status."))
			}

			_, client, err := newClient()
			if err != nil {
				return fail(err)
			}

			// Read-modify-write: fetch the current page (with its body in the
			// target representation) so we can preserve unchanged fields and
			// compute the next version number.
			getPath := fmt.Sprintf("/pages/%s?body-format=%s", confluence.EncodePathSegment(id), upFormat)
			var current map[string]any
			if err := client.Request(ctx(cmd), getPath, confluence.RequestOptions{}, &current); err != nil {
				return fail(err)
			}

			nextVersion, err := nextVersionNumber(current)
			if err != nil {
				return fail(err)
			}

			payload := map[string]any{
				"id":     id,
				"status": pickString(statusProvided, upStatus, current, "status", "current"),
				"title":  pickString(titleProvided, upTitle, current, "title", ""),
				"version": map[string]any{
					"number": nextVersion,
				},
			}
			if strings.TrimSpace(upMsg) != "" {
				payload["version"].(map[string]any)["message"] = upMsg
			}
			if bodyProvided {
				payload["body"] = map[string]any{"representation": upFormat, "value": body}
			} else {
				payload["body"] = map[string]any{"representation": upFormat, "value": currentBodyValue(current, upFormat)}
			}

			var raw json.RawMessage
			putPath := fmt.Sprintf("/pages/%s", confluence.EncodePathSegment(id))
			if err := client.Request(ctx(cmd), putPath, confluence.RequestOptions{Method: http.MethodPut, Body: payload}, &raw); err != nil {
				return fail(err)
			}
			if err := emitObject(raw, output.PageSummary); err != nil {
				return fail(err)
			}
			return nil
		},
	}
	updateCmd.Flags().StringVar(&upTitle, "title", "", "New page title")
	updateCmd.Flags().StringVar(&upStatus, "status", "", "New status: current or draft")
	updateCmd.Flags().StringVar(&upBody, "body", "", "New body in the chosen representation")
	updateCmd.Flags().StringVar(&upBodyF, "body-file", "", "Read body from file (- for stdin)")
	updateCmd.Flags().StringVar(&upFormat, "body-format", "storage", "Body representation: storage or atlas_doc_format")
	updateCmd.Flags().StringVar(&upMsg, "version-message", "", "Optional version change message")

	pageCmd.AddCommand(createCmd, updateCmd)
}

// nextVersionNumber returns current.version.number + 1.
func nextVersionNumber(page map[string]any) (int, error) {
	version, ok := page["version"].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("page response missing version object")
	}
	n, ok := version["number"].(float64)
	if !ok {
		return 0, fmt.Errorf("page response missing version.number")
	}
	return int(n) + 1, nil
}

// pickString returns the override when provided, else the page's stored value
// for key, else fallback.
func pickString(provided bool, override string, page map[string]any, key, fallback string) string {
	if provided {
		return override
	}
	if s, ok := page[key].(string); ok && s != "" {
		return s
	}
	return fallback
}

// currentBodyValue extracts the page's body value in the given representation.
func currentBodyValue(page map[string]any, format string) string {
	body, ok := page["body"].(map[string]any)
	if !ok {
		return ""
	}
	if rep, ok := body[format].(map[string]any); ok {
		if v, ok := rep["value"].(string); ok {
			return v
		}
	}
	return ""
}
