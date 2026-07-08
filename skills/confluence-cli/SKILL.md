---
name: confluence-cli
description: "Drive Confluence Cloud (spaces, pages, comments, attachments) from the command line via the `confluence-cli` tool. Use when the user asks to read/search/list/create/update Confluence pages, read or post page comments, inspect spaces, or otherwise interact with Confluence Cloud."
---

# Driving Confluence Cloud with `confluence-cli`

`confluence-cli` is a standalone Go CLI for Confluence Cloud built on the REST v2
API. It is built for agents: **output is JSON on stdout by default** (parse it
directly), and `--pretty` switches to one-line human text. Errors are a JSON
envelope on stderr with a non-zero exit code.

## Before you start: verify config

Credentials resolve in this order: **env vars → config file**. There is no git
auto-detection; a **site** must be configured.

```bash
confluence-cli status        # reports config validity, site, and default space
```

If `status` reports missing config, set an Atlassian API token and site:

```bash
confluence-cli config set email you@example.com
confluence-cli config set api_token <atlassian-api-token>       # never echo or log this
confluence-cli config set site https://your-site.atlassian.net  # normalized to host
confluence-cli config set default_space ENG                     # optional (key or id)
confluence-cli config list   # token is redacted
```

The token needs `read:page:confluence`, `read:space:confluence`,
`read:comment:confluence`, and (for writes) `write:page:confluence` /
`write:comment:confluence`. Override the space per command with `--space <key|id>`.

## Read commands (safe — run freely)

```bash
confluence-cli space list --type global          # types: global | personal
confluence-cli space get ENG                      # by key or numeric id
confluence-cli page list --space ENG              # pages in a space
confluence-cli page list --space ENG --title "Runbook"
confluence-cli page get 12345 --body-format storage   # storage | atlas_doc_format | view
confluence-cli page comments 12345                # footer comments
confluence-cli attachment list 12345              # attachments on a page
```

Pipe JSON into `jq`, e.g. `confluence-cli page get 12345 --body-format storage | jq -r '.body.storage.value'`.

## Body format (important)

Page and comment bodies are sent **verbatim** — the CLI does **not** convert
markdown. Supply valid content for the chosen `--body-format`:
- `storage` (default): Confluence storage XHTML, e.g. `<p>Hello <strong>world</strong></p>`.
- `atlas_doc_format`: ADF JSON.
- `view`: read-only (only valid for `page get`).

## Write commands (only when the user explicitly asks)

These mutate Confluence. **Do not run them speculatively.** State what you are
about to do first.

```bash
# Post a footer comment, or a threaded reply
confluence-cli comment create --page 12345 --body "<p>Thanks, taking a look.</p>"
confluence-cli comment create --page 12345 --body "<p>Fixed.</p>" --reply-to 999

# Create a page (space defaults to --space / default_space)
confluence-cli page create --space ENG --title "Deploy Runbook" --parent 12345 \
  --body-file runbook.xhtml
# long bodies via stdin:
render-storage | confluence-cli page create --space ENG --title "Notes" --body-file -

# Update a page. Confluence bumps the version and re-sends title/body/status,
# so the command reads the page first and preserves whatever you don't change.
confluence-cli page update 12345 --title "Deploy Runbook (v2)"
confluence-cli page update 12345 --body-file runbook.xhtml
```

`--body` and `--body-file` are mutually exclusive; `--body-file -` reads stdin.

## Global flags & output contract

- `--space <key|id>`: target a space for one command (overrides `default_space`).
- `--pretty`: one-line text summaries instead of JSON (for showing the user, not parsing).
- **List commands** emit a JSON array of full v2 objects; **single-entity** commands emit one object.
- **Errors** → stderr as `{"error":{"message":...,"status":...,"method":...,"url":...,"excerpt":...}}`, exit 1. Check the exit code and surface `.error.message`.

## Tips for agents

- Default to JSON; only add `--pretty` when presenting results to a human.
- Run `confluence-cli status` before assuming a site/space is configured.
- Space keys (e.g. `ENG`) and numeric ids are interchangeable in `--space` and `space get`.
- Never print or commit the API token, `.env`, or a config file containing a real token.
- If a command fails, read the JSON error envelope on stderr — `status` and `excerpt`
  usually explain why (auth, missing scope, wrong site/space/id, version conflict).
- **No attachment upload** and **no CQL search** in this tool (v2 API limits); use
  `page list --title` and `attachment list` for discovery.
