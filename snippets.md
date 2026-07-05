---
name: Starter Snippets
tags: [starter, example]
---

## List directory contents

Show detailed directory listing.

```sh
ls -lsha <$PATH:?./>
```

## Curl JSON endpoint

Post a request body to a URL.

```bash
curl -X POST -H "Content-Type: application/json" -d @<$FILE_NAME> "<$CURL_URL>"
```

## Curl JSON endpoint Custom Method

Post a request body to a URL with any method

```bash
curl -X <$CURL_METHOD:?GET|POST|PUT|DELETE> -H "Content-Type: application/json" -d @<$FILE_NAME> "<$CURL_URL>"
```
