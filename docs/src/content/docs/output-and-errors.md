---
title: Output and errors
description: Integrate confluence-cli reliably into scripts and agent workflows.
---

## JSON output

Commands emit JSON by default. List commands return arrays of full Confluence v2
objects; single-resource commands return the entity object. Pretty output is
opt-in:

```sh
confluence-cli page list          # JSON on stdout
confluence-cli page list --pretty # one summary line per page
```

Errors are written to stderr:

```json
{
  "error": {
    "message": "Confluence resource not found.",
    "status": 404,
    "method": "GET",
    "url": "https://example.atlassian.net/wiki/api/v2/pages/123",
    "excerpt": "..."
  }
}
```

Configuration and usage errors contain only a message. HTTP errors include
status, method, URL, and a bounded response excerpt.

## Exit behavior

- Exit `0` indicates success.
- Exit `1` indicates an API, network, authentication, configuration, or usage
  failure.

Parse stdout and stderr separately. `page md` is the exception to normal stdout
formatting: it emits raw Markdown on success while still using the JSON error
envelope on failure.

## Failure handling

Authentication, authorization, not-found, rate-limit, and network errors are
surfaced without automatic retries. Correct the site, credentials, permissions,
or request parameters before trying again. Remote page/comment writes are not
safe to retry blindly.
