package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/dtonair/confluence-cli/confluence"
	"github.com/dtonair/confluence-cli/markdown"

	"github.com/spf13/cobra"
)

// frontmatterOrder fixes the emission order of YAML frontmatter keys. Keys absent
// from the metadata map (or empty) are skipped by markdown.Render.
var frontmatterOrder = []string{"title", "id", "space", "spaceId", "status", "version", "url"}

func init() {
	var (
		mdFrontmatter bool
		mdOutput      string
	)
	mdCmd := &cobra.Command{
		Use:   "md <id>",
		Short: "Export a page as Markdown",
		Long: "Fetch a page's rendered (view) body and convert it to GitHub-Flavored " +
			"Markdown. Unlike other commands, this prints raw Markdown (not JSON) to " +
			"stdout; --pretty is ignored. Errors still use the JSON error envelope.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("page id", args[0])
			if err != nil {
				return fail(err)
			}
			_, client, err := newClient()
			if err != nil {
				return fail(err)
			}

			// Fetch the rendered HTML body.
			var page map[string]any
			path := fmt.Sprintf("/pages/%s?body-format=view", confluence.EncodePathSegment(id))
			if err := client.Request(ctx(cmd), path, confluence.RequestOptions{}, &page); err != nil {
				return fail(err)
			}

			body, err := markdown.FromHTML(viewBodyValue(page))
			if err != nil {
				return fail(err)
			}

			out := body
			if mdFrontmatter {
				meta := buildFrontmatter(cmd, client, page)
				out = markdown.Render(body, meta, frontmatterOrder)
			}

			return writeMarkdown(mdOutput, out)
		},
	}
	mdCmd.Flags().BoolVar(&mdFrontmatter, "frontmatter", false, "Prepend a YAML frontmatter block (title, id, space, status, version, url)")
	mdCmd.Flags().StringVarP(&mdOutput, "output", "o", "", "Write Markdown to a file instead of stdout (- for stdout)")

	pageCmd.AddCommand(mdCmd)
}

// viewBodyValue extracts page.body.view.value, or "" when absent.
func viewBodyValue(page map[string]any) string {
	body, ok := page["body"].(map[string]any)
	if !ok {
		return ""
	}
	if view, ok := body["view"].(map[string]any); ok {
		if v, ok := view["value"].(string); ok {
			return v
		}
	}
	return ""
}

// buildFrontmatter assembles the metadata map from the page JSON. The space key
// requires a reverse lookup (GET /spaces/{id}); it is best-effort and omitted on
// any error so a cosmetic field never fails the command.
func buildFrontmatter(cmd *cobra.Command, client *confluence.Client, page map[string]any) map[string]any {
	meta := map[string]any{}
	if s, ok := page["title"].(string); ok {
		meta["title"] = s
	}
	if s, ok := page["id"].(string); ok {
		meta["id"] = s
	}
	spaceID, _ := page["spaceId"].(string)
	if spaceID != "" {
		meta["spaceId"] = spaceID
		if key := lookupSpaceKey(cmd, client, spaceID); key != "" {
			meta["space"] = key
		}
	}
	if s, ok := page["status"].(string); ok {
		meta["status"] = s
	}
	if version, ok := page["version"].(map[string]any); ok {
		if n, ok := version["number"].(float64); ok {
			meta["version"] = int(n)
		}
	}
	if u := pageURL(page); u != "" {
		meta["url"] = u
	}
	return meta
}

// lookupSpaceKey resolves a numeric space id to its key via GET /spaces/{id}.
// Returns "" on any error (best-effort; never fails the caller).
func lookupSpaceKey(cmd *cobra.Command, client *confluence.Client, spaceID string) string {
	var space map[string]any
	path := fmt.Sprintf("/spaces/%s", confluence.EncodePathSegment(spaceID))
	if err := client.Request(ctx(cmd), path, confluence.RequestOptions{}, &space); err != nil {
		return ""
	}
	if key, ok := space["key"].(string); ok {
		return key
	}
	return ""
}

// pageURL assembles the browser URL from _links.base + _links.webui when both
// are present; returns "" otherwise.
func pageURL(page map[string]any) string {
	links, ok := page["_links"].(map[string]any)
	if !ok {
		return ""
	}
	base, _ := links["base"].(string)
	webui, _ := links["webui"].(string)
	if base == "" || webui == "" {
		return ""
	}
	return strings.TrimRight(base, "/") + webui
}

// writeMarkdown writes content (with a single trailing newline) to stdout when
// dest is "" or "-", otherwise to the named file. On file output, nothing is
// printed to stdout.
func writeMarkdown(dest, content string) error {
	payload := content + "\n"
	if dest == "" || dest == "-" {
		if _, err := fmt.Fprint(os.Stdout, payload); err != nil {
			return fail(err)
		}
		return nil
	}
	if err := os.WriteFile(dest, []byte(payload), 0644); err != nil {
		return fail(fmt.Errorf("write markdown to %q: %w", dest, err))
	}
	return nil
}
