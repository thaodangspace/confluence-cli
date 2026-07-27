---
title: Getting started
description: Install confluence-cli, configure a site, and make a read-only request.
---

## Requirements

- Go, if installing or building from source
- An Atlassian API token and the associated email address
- A Confluence Cloud site and access to its spaces/pages

## Install

Install the latest tagged release with Go:

```sh
go install github.com/dtonair/confluence-cli@latest
```

You can also download a release tarball or build from source:

```sh
go build -o confluence-cli .
```

Verify the binary:

```sh
confluence-cli --help
confluence-cli --version
```

## Configure credentials and site

For a shell session, set the required values:

```sh
export CONFLUENCE_EMAIL="you@example.com"
export CONFLUENCE_API_TOKEN="your-atlassian-api-token"
export CONFLUENCE_SITE="your-site.atlassian.net"
export CONFLUENCE_DEFAULT_SPACE="ENG" # optional
```

`CONFLUENCE_SITE` accepts a bare host or a full URL such as
`https://your-site.atlassian.net/wiki`; it is normalized to the site host.

Alternatively, create `~/.config/confluence-cli.yaml`:

```yaml
email: you@example.com
api_token: your-atlassian-api-token
site: your-site.atlassian.net
default_space: ENG
```

Environment variables take precedence over the config file. Unlike
`bitbucket-cli`, Confluence has no git-remote auto-detection.

## Make a read-only request

Check configuration without changing Confluence:

```sh
confluence-cli status
```

Then list global spaces using JSON output:

```sh
confluence-cli space list --type global
```

See [Commands](/commands/) and [Configuration and security](/configuration-security/)
for the remaining options and credential rules.
