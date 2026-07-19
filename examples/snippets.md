---
name: Starter Snippets
tags: [starter, example]
variables:
  PATH:
    default: "."
  CURL_METHOD:
    default: GET
    suggestions: [GET, POST, PUT, DELETE]
  FILE_NAME:
    default: /tmp/sample.json
    suggestions: ["$(ls /tmp/*.json 2>/dev/null || echo /tmp/sample.json)"]
---

## List directory contents

Show detailed directory listing.

```sh
ls -lsha <$PATH>
```

## Curl JSON endpoint

Post a request body to a URL.

```bash
curl -X POST -H "Content-Type: application/json" -d @<$FILE_NAME> "<$CURL_URL>"
```

## Curl JSON endpoint Custom Method

Post a request body to a URL with any method

```bash
curl -X <$CURL_METHOD> -H "Content-Type: application/json" -d @<$FILE_NAME> "<$CURL_URL>"
```
