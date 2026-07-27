# AGENTS.md — confluence-cli

Standalone Go CLI for Confluence Cloud (spaces, pages, comments, attachments)
over the REST **v2** API (`/wiki/api/v2`). A sibling to `bitbucket-cli` and
`asana-cli`, sharing their output contract, config precedence, and safety
conventions so any agent/script can drive Confluence.

## Layout

| Path | Responsibility |
| --- | --- |
| `main.go` | Entry point → `cmd.Execute()` |
| `cmd/` | Cobra commands. One file per area: `space.go`, `page.go` (read), `page_md.go` (Markdown export), `page_write.go` + `comment.go` (write), `attachment.go`, `status.go`, `config.go`, `root.go`. `common.go` holds shared helpers. |
| `confluence/client.go` | Thin REST v2 client: Basic auth, JSON `Request`, `Paginate`, normalized `HTTPError`. |
| `config/config.go` | Config resolution: env → YAML file. Site normalization; **no git auto-detect**. |
| `markdown/` | Pure-Go HTML→Markdown core (`FromHTML`, `Render`) wrapping `html-to-markdown` v1.6.0 + GFM plugin. Network-free; unit-tested independently. |
| `output/` | `RenderJSON`/`RenderLines`/`WriteError` and `*Summary` text formatters. |
| `docs/` | Astro/Starlight static documentation site. |

## Conventions

- **Output contract**: JSON on stdout by default; `--pretty` emits one-line text
  summaries (`output.*Summary`). Errors go to stderr as a JSON envelope, exit 1.
  Use `emitObject` / `emitList` (in `cmd/common.go`) so every command renders the
  same way.
- **Read vs write commands**: write commands (`page create`, `page update`,
  `comment create`) must carry a `Long` description stating they are write
  operations to run only when the user explicitly asked. Deliberate safety marker.
- **HTTP**: no per-command HTTP code. Call `client.Request(ctx, path,
  confluence.RequestOptions{Method, Body}, &out)` for JSON APIs — GET/POST/PUT with
  a JSON body, `*HTTPError` for non-2xx (surfaces method/url/status/excerpt). For
  lists, call `client.Paginate(ctx, path, limit, maxPages)`.
- **Shared helpers** (`cmd/common.go`): `newClient`, `resolveSpaceID` /
  `spaceIDFor` (key→numeric-id lookup), `parseID` (numeric string), `ctx`, `fail`,
  `emitObject`, `emitList`.
- **Body input** (`cmd/page_write.go`): `readBody(text, file, stdin)` resolves a
  body from `--body` / `--body-file` (`-` = stdin); the two sources are mutually
  exclusive. Reuse it for any future text-body flag. Bodies are passed through
  **verbatim** — no markdown conversion on *write*.
- **Markdown export** (`cmd/page_md.go` + `markdown/`): `page md <id>` is the one
  command that emits **raw Markdown** to stdout, not the JSON envelope (`--pretty`
  ignored; errors still use the envelope). It fetches `body-format=view` and calls
  `markdown.FromHTML`. `--frontmatter` builds a metadata map and calls
  `markdown.Render`; the human space **key** is a best-effort `GET /spaces/{id}` that
  degrades to `spaceId`-only on error (a cosmetic field must never fail the command).
  Conversion is **view-only** by design; `storage`/ADF sources are a deliberate
  follow-up. Adds the `html-to-markdown` dep (pinned v1.6.0 — light tree, go 1.13
  compat; v2 would force a go 1.25 toolchain bump).

## Site & base URL (differs from bitbucket-cli)

There is no workspace/repo and no git detection. The target is a **site**
(`CONFLUENCE_SITE`, a host or full URL, normalized to a host by
`config.NormalizeSite`). The API base is `https://<site>/wiki/api/v2`.
`client.buildURL` handles three path shapes:
- absolute `https://…` → used verbatim;
- site-relative `/wiki/…` (what `_links.next` returns) → `https://<site>` + path;
- API-relative `/pages` etc. → base + path.

## Pagination (differs from bitbucket-cli)

Confluence v2 uses `{results: [...], _links: {next: "/wiki/api/v2/…?cursor=…"}}`,
**not** Bitbucket's `{values, next}`. `Paginate` accumulates `results` and follows
the site-relative `_links.next` until `--limit` or `DefaultMaxPages` is hit.

## Space id vs key

Spaces have both a numeric `id` and an alphanumeric `key`. `--space` and
`space get <id|key>` accept either. `spaceIDFor` returns numeric input as-is and
resolves a key via `GET /spaces?keys={key}` (`results[0].id`). Page-scoped
endpoints (`GET /spaces/{id}/pages`) require the numeric id.

## Page update rule (important)

Confluence `PUT /pages/{id}` requires `id`, `status`, `title`, `body`, and
`version.number` = **current + 1**; omitting `title`/`body` can blank them.
`page update` therefore does a **read-modify-write**: GET the page (with its body
in the target representation), carry over `title`/`body`/`status` unless the
matching flag was set, compute the next version number, then PUT. Preserve this
for any future page mutation.

## Body representation

`--body-format` selects the representation: `storage` (default, XHTML),
`atlas_doc_format` (ADF JSON), or `view` (read-only; rejected on create/update).
`page get` requests the body in that representation.

## Provenance / divergences from bitbucket-cli

`confluence-cli` is modeled on `bitbucket-cli`'s architecture, not a port of a
specific upstream extension. Intentional divergences:
- `--space` (key or id) replaces `--workspace`/`--repo`; required `site`.
- env → file only (no git remote auto-detection).
- `results`/`_links.next` cursor pagination.
- version-bump read-modify-write for `page update`.
- **No attachment upload**: the v2 API has no create-attachment endpoint (upload
  is v1-only `/wiki/rest/api`). `attachment list` is read-only.
- Footer comments only (no inline comments); no CQL search, page delete, labels,
  or permissions in this cut.

## Testing

- `go test ./...`. Tests stub HTTP via a `roundTripFunc` `testTransport` injected
  by the `run(t, transport, args...)` helper in `cmd/commands_test.go`.
- `run` resets global + per-command flag state (`resetFlags`) because cobra reuses
  the `rootCmd` singleton across `Execute()` calls — flag values and `Changed`
  markers leak between tests otherwise.
- `run` sandboxes `CONFLUENCE_CONFIG` to a temp path so tests never read the
  developer's real `~/.config/confluence-cli.yaml`.

## Build

`go build -o confluence-cli .` or `go install .`. Deps: `spf13/cobra`,
`gopkg.in/yaml.v3` only. Release via goreleaser (`.goreleaser.yaml`).

The docs site uses npm from `docs/` and emits static output to `docs/dist/`:
`make docs-build`. Cloudflare Pages uses root `docs`, build command
`npm run build`, and output `dist`.
