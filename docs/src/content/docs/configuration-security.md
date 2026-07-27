---
title: Configuration and security
description: Manage Confluence credentials, site selection, body formats, and write boundaries.
---

## Credential precedence

Configuration resolves in this order:

1. `CONFLUENCE_EMAIL`, `CONFLUENCE_API_TOKEN`, `CONFLUENCE_SITE`, and `CONFLUENCE_DEFAULT_SPACE`
2. `~/.config/confluence-cli.yaml` (or `$XDG_CONFIG_HOME/confluence-cli.yaml`)

Set `CONFLUENCE_CONFIG` to override the config-file path. The `config` command
can write and inspect values without printing the API token:

```sh
confluence-cli config set email you@example.com
confluence-cli config set api_token your-atlassian-api-token
confluence-cli config set site https://your-site.atlassian.net/wiki
confluence-cli config list
```

Restrict the config file to your user and never commit it:

```sh
chmod 600 ~/.config/confluence-cli.yaml
```

## API boundary and body formats

Requests target `https://<site>/wiki/api/v2` and use Basic authentication with
the configured email and Atlassian API token. There is no git remote
auto-detection and no user-facing alternate API base URL.

Page and comment bodies are sent verbatim in their selected representation:

- `storage` (default): Confluence storage XHTML
- `atlas_doc_format`: Atlassian Document Format JSON
- `view`: read-only for `page get`; rejected for writes

The reverse conversion is `page md`, which converts rendered `view` HTML to
Markdown. This CLI does not upload attachments; Confluence v2 has no create-
attachment endpoint in the supported API boundary.

## Safe page updates

`page update` performs a read-modify-write because Confluence requires the page
ID, status, title, body, and `version.number` incremented from the current page.
It carries forward fields that were not changed so an update does not blank page
content or discard the title.

:::danger[Protect credentials and writes]
Keep tokens out of shell history, source control, and documentation. Treat
`page create`, `page update`, and `comment create` as remote mutations and run
them only after explicit authorization.
:::

The test suite injects an HTTP transport and does not require network access or
real credentials.
