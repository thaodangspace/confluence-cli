package output

import (
	"fmt"
	"strings"
)

// These helpers render the one-line text summaries emitted under --pretty,
// operating on decoded JSON objects (map[string]any) from the Confluence v2 API.

func str(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func strOr(m map[string]any, key, fallback string) string {
	if s, ok := str(m, key); ok && s != "" {
		return s
	}
	return fallback
}

// SpaceSummary renders "<key> — <name> [<type>]".
func SpaceSummary(space map[string]any) string {
	key := strOr(space, "key", "?")
	name := strOr(space, "name", "(unnamed)")
	typ := strOr(space, "type", "unknown")
	return fmt.Sprintf("%s — %s [%s]", key, name, typ)
}

// PageSummary renders "<id> <title> [<status>]" with an optional version and
// space suffix when those fields are present on the object.
func PageSummary(page map[string]any) string {
	id := strOr(page, "id", "?")
	title := strOr(page, "title", "(untitled)")
	status := strOr(page, "status", "unknown")
	out := fmt.Sprintf("%s %s [%s]", id, title, status)
	if version, ok := page["version"].(map[string]any); ok {
		if n, ok := version["number"]; ok {
			out += fmt.Sprintf(" v%v", numberish(n))
		}
	}
	if spaceID := strOr(page, "spaceId", ""); spaceID != "" {
		out += fmt.Sprintf(" (space %s)", spaceID)
	}
	return out
}

// CommentSummary renders "#<id>: <first line of body>".
func CommentSummary(comment map[string]any) string {
	id := strOr(comment, "id", "?")
	body := commentBodyText(comment)
	// Collapse to a single line and trim.
	if i := strings.IndexAny(body, "\r\n"); i >= 0 {
		body = body[:i]
	}
	body = strings.TrimSpace(body)
	if len(body) > 120 {
		body = body[:120] + "…"
	}
	return fmt.Sprintf("#%s: %s", id, body)
}

// commentBodyText extracts the body value from whichever representation the API
// returned (storage, atlas_doc_format, or view).
func commentBodyText(comment map[string]any) string {
	body, ok := comment["body"].(map[string]any)
	if !ok {
		return ""
	}
	for _, rep := range []string{"storage", "view", "atlas_doc_format"} {
		if r, ok := body[rep].(map[string]any); ok {
			if v, ok := str(r, "value"); ok {
				return v
			}
		}
	}
	return ""
}

// AttachmentSummary renders "<title> (<mediaType>, <fileSize> bytes)".
func AttachmentSummary(att map[string]any) string {
	title := strOr(att, "title", "(untitled)")
	mediaType := strOr(att, "mediaType", "unknown")
	size := "?"
	if v, ok := att["fileSize"]; ok {
		size = fmt.Sprintf("%v", numberish(v))
	}
	return fmt.Sprintf("%s (%s, %s bytes)", title, mediaType, size)
}

// numberish normalizes JSON numbers (float64) to integers for display when they
// are whole, so a version renders as "3" not "3".
func numberish(v any) any {
	if f, ok := v.(float64); ok && f == float64(int64(f)) {
		return int64(f)
	}
	return v
}
