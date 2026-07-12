# confluence-cli

A standalone Go CLI for Confluence Cloud spaces, pages, and comments, built on
the [Confluence Cloud REST API v2](https://developer.atlassian.com/cloud/confluence/rest/v2/intro/).
It is a sibling to `bitbucket-cli` and `asana-cli`, built so any agent or script
can drive Confluence without a bespoke integration.

Output is **JSON by default** (easy for agents to parse); pass `--pretty` for
human-readable text. Errors are written as a JSON envelope to stderr with a
non-zero exit code.

## Install

### go install

```bash
go install github.com/dtonair/confluence-cli@latest
```

Installs the latest tagged release into `$GOBIN`. Requires Go 1.24+. Pin a
specific version with `@v0.1.0`.

### Prebuilt binary

Download a tarball for your OS/arch from the
[Releases page](https://github.com/dtonair/confluence-cli/releases), extract it,
and put `confluence-cli` on your `PATH`. No Go toolchain required.

### Build from source

```bash
cd ~/code/confluence-cli
go build -o confluence-cli .      # local binary
# or
go install .                      # into $GOBIN
```

Third-party dependencies are kept minimal: `spf13/cobra` and `gopkg.in/yaml.v3`,
plus `JohannesKaufmann/html-to-markdown` (pinned to v1.6.0 for its light, Go
1.13-compatible dependency tree) for the `page md` HTML→Markdown conversion.

## Configuration

Credentials and defaults are resolved with the following precedence:
**environment variables → config file**. (Unlike `bitbucket-cli`, there is no git
auto-detection — Confluence is addressed by an explicit site.)

### Environment variables

```bash
export CONFLUENCE_EMAIL="you@example.com"
export CONFLUENCE_API_TOKEN="your-atlassian-api-token"
export CONFLUENCE_SITE="your-site.atlassian.net"     # host or full URL
export CONFLUENCE_DEFAULT_SPACE="ENG"                 # optional: key or numeric id
```

`CONFLUENCE_SITE` accepts a bare host (`your-site.atlassian.net`) or a full URL
(`https://your-site.atlassian.net/wiki`); it is normalized to a host. The API
base becomes `https://<site>/wiki/api/v2`.

### Config file

Any value not set in the environment is read from a YAML config file at
`~/.config/confluence-cli.yaml` (or `$XDG_CONFIG_HOME/confluence-cli.yaml`).
Override the path with `CONFLUENCE_CONFIG`.

```yaml
# ~/.config/confluence-cli.yaml
email: you@example.com
api_token: your-atlassian-api-token
site: your-site.atlassian.net
default_space: ENG          # optional
```

Manage the file with the `config` command instead of editing it by hand:

```bash
confluence-cli config set email you@example.com
confluence-cli config set api_token your-atlassian-api-token
confluence-cli config set site https://your-site.atlassian.net/wiki   # normalized to host
confluence-cli config set default_space ENG
confluence-cli config get site
confluence-cli config list          # API token redacted
confluence-cli config path          # print the resolved file path
```

The token must be an Atlassian API token for an account with access to the target
site. Recommended scopes: `read:page:confluence`, `read:space:confluence`,
`read:comment:confluence`, and (for writes) `write:page:confluence` and
`write:comment:confluence`.

## Commands

| Command | Description |
| --- | --- |
| `status` | Report config validity, site, and default space |
| `config set/get/list/path` | Manage the config file (token redacted in `list`) |
| `space list [--keys k1,k2] [--type global\|personal] [--status current\|archived] [--limit N]` | List spaces |
| `space get <id\|key>` | One space (resolves a key to its id) |
| `page list [--space <key\|id>] [--title <t>] [--status current\|draft\|archived] [--limit N]` | List pages (scoped to a space when one is set) |
| `page get <id> [--body-format storage\|atlas_doc_format\|view]` | One page |
| `page md <id> [--frontmatter] [-o\|--output <file>]` | Export a page as GitHub-Flavored Markdown (converts the rendered `view` body) |
| `page comments <id> [--limit N]` | Footer comments on a page |
| `attachment list <page-id> [--limit N]` | Attachments on a page |
| `page create --title <t> [--space <key\|id>] [--parent <id>] [--body ...\|--body-file ...] [--body-format ...]` | Create a page (write) |
| `page update <id> [--title <t>] [--body ...\|--body-file ...] [--status ...] [--body-format ...]` | Update a page (write) |
| `comment create --page <id> --body <s> [--reply-to <comment-id>] [--body-format ...]` | Post a footer comment or reply (write) |

Global flags: `--space`, `--pretty`. Default `--limit` is 20.

## Examples

```bash
# JSON (default) — for agents
confluence-cli space list --type global
confluence-cli page list --space ENG --title "Runbook"
confluence-cli page get 12345 --body-format storage | jq -r '.body.storage.value'

# Export a page to Markdown (converts the rendered view body — no pandoc needed)
confluence-cli page md 12345 > runbook.md
confluence-cli page md 12345 --frontmatter -o runbook.md   # with YAML metadata header

# Human-readable
confluence-cli page list --space ENG --pretty
# -> 12345 Runbook [current] v3 (space 777)

# Post a comment or reply (write — only run when explicitly asked)
confluence-cli comment create --page 12345 --body "<p>Thanks, taking a look.</p>"
confluence-cli comment create --page 12345 --body "<p>Fixed.</p>" --reply-to 999

# Create a page under a parent (write); body is verbatim storage XHTML
confluence-cli page create --space ENG --title "Deploy Runbook" --parent 12345 \
  --body-file runbook.xhtml
# long bodies via stdin:
render-storage | confluence-cli page create --space ENG --title "Notes" --body-file -

# Update a page's body (write); version is bumped and title/status preserved
confluence-cli page update 12345 --body-file runbook.xhtml
confluence-cli page update 12345 --title "Deploy Runbook (v2)"

# Target a specific space for one command
confluence-cli --space DOCS page list
```

## Body formats (important)

Page and comment bodies are sent **verbatim** in the chosen representation — the
CLI does **not** convert markdown *on write*. Supply valid content for the format:

- `storage` (default) — Confluence storage format (XHTML).
- `atlas_doc_format` — Atlassian Document Format (ADF JSON).
- `view` — read-only, valid for `page get` only (rejected on create/update).

The reverse direction — Confluence → Markdown — is available via `page md`, which
fetches the rendered `view` body and converts it to GitHub-Flavored Markdown in pure
Go (no `pandoc` required). Note this is the one command that prints **raw Markdown**
to stdout rather than the JSON envelope (`--pretty` is ignored); errors still use the
JSON error envelope. Conversion is view-only for now — `storage`/`atlas_doc_format`
sources and image/attachment downloading are out of scope.

## Output contract

- **Default**: JSON on stdout. List commands emit a JSON array of the full v2
  objects; single-entity commands emit the entity object.
- **`--pretty`**: one-line text summaries.
- **Errors**: JSON `{"error":{"message":...,"status":...,"method":...,"url":...,"excerpt":...}}`
  on stderr, exit code 1. Config/usage errors carry only `message`.

## Security

Credentials are read from environment variables or the config file and are never
logged or echoed. Do not commit tokens, `.env` files, or a config file containing
a real API token (`chmod 600` it and keep it out of version control).

## Testing

```bash
go test ./...
```
