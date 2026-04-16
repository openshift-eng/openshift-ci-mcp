---
name: sippy-api-explorer
description: Probe a Sippy API endpoint to discover required params, response shape, and filter format
---

Investigate a Sippy API endpoint and report its contract. The base URL is `https://sippy.dptools.openshift.org`.

## Process

1. **Call with no params** to see what's required (expect 400 errors listing required params).
2. **Call with required params only** to get a baseline response.
3. **Test optional params** one at a time to see their effect.
4. **Check filter support** — try adding a `filter` query param with JSON like:
   ```
   {"items":[{"columnField":"name","operatorValue":"contains","value":"test"}],"linkOperator":"and"}
   ```
5. **Check pagination** — try `perPage`, `page`, `limit`, `sort`, `sortField` params.

## Output

Report a concise contract:

```
Endpoint: /api/example
Required params: release
Optional params: perPage (default 25), page (default 1), sortField, sort (asc/desc)
Filter fields: name, org, repo (via filter JSON param)
Response shape: [] array of objects with fields: {id, name, passPercentage, ...}
Notes: any gotchas discovered
```

Use `curl -s` with short timeouts (`--max-time 15`). Parse responses with `python3 -c "import sys,json; ..."` for readability. Use release `4.18` for testing unless told otherwise.
