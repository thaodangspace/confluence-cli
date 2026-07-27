---
title: Commands
description: Inspect Confluence content, export Markdown, and perform explicit page writes.
---

## Global options

- `--space KEY_OR_ID` selects a default space for the command.
- `--pretty` prints one-line summaries instead of JSON.
- List commands default to a limit of 20; pagination is bounded internally.

## Configuration and spaces

```sh
confluence-cli status
confluence-cli config path
confluence-cli config get <key>
confluence-cli config set <key> <value>
confluence-cli config list
confluence-cli space list [--keys K1,K2] [--type global|personal] [--status current|archived] [--limit N]
confluence-cli space get <id-or-key>
```

A space can be addressed by its numeric ID or alphanumeric key. Key-based page
endpoints are resolved to the numeric ID first.

## Pages and related content

```sh
confluence-cli page list [--space KEY_OR_ID] [--title TITLE] [--status current|draft|archived] [--limit N]
confluence-cli page get <id> [--body-format storage|atlas_doc_format|view]
confluence-cli page comments <id> [--limit N]
confluence-cli attachment list <page-id> [--limit N]
```

Confluence v2 pagination uses a cursor in `_links.next`; the CLI follows it up
to its configured page cap.

## Markdown export

```sh
confluence-cli page md <id>
confluence-cli page md <id> --frontmatter -o runbook.md
```

`page md` fetches the rendered `view` body and converts it to GitHub-Flavored
Markdown in pure Go. It is the one command that writes raw Markdown to stdout
rather than the normal JSON output; `--pretty` is ignored. `--frontmatter`
adds page metadata. Conversion is view-only: storage and ADF source conversion,
plus image/attachment downloading, are not included.

## Page and comment writes

These commands modify Confluence and should run only when explicitly requested:

```sh
confluence-cli page create --space ENG --title "Runbook" --body-file runbook.xhtml
confluence-cli page update <id> --body-file runbook.xhtml
confluence-cli comment create --page <id> --body "<p>Thanks.</p>"
```

`page create` accepts `--parent`, `--body`, `--body-file`, and `--body-format`.
`page update` accepts title, body, status, and body-format flags. Body sources are
mutually exclusive and are passed verbatim; the CLI does not convert Markdown on
write. `comment create` supports `--reply-to` and body format selection.

:::danger[Write safety]
Do not run `page create`, `page update`, or `comment create` unless the user
explicitly asked for the remote change.
:::
