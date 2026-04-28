---
title: "Node.js https.get crash on malformed 3xx redirect without Location header"
description: "Node.js https.get() silently passes undefined to a recursive call when a 3xx redirect is missing the Location header, causing an opaque crash and socket leak."
problem_type: runtime_error
category: runtime_error
component: cli
root_cause: incorrect_error_type
resolution_type: code_fix
severity: high
message: "https.get() passes undefined to recursive get() call when 3xx response lacks Location header, causing an opaque crash instead of a descriptive error."
file_path: "npm/backlogit-mcp/lib/resolve.js"
resolved: true
tags: [nodejs, https, redirect, undefined, crash, fetch, binary-resolver, socket-leak]
date: 2026-04-28
---

## Problem

`fetchBuf()` follows HTTP redirects by recursively calling `get(headers.location)`.
When the server returns a 3xx status with no `Location` header, `headers.location`
is `undefined`. Node.js `https.get()` does not validate its URL argument — it
accepts `undefined` and throws an opaque internal error downstream rather than
a descriptive one.

The same pattern occurs with `http.get()`.

## Symptoms

- Tier 3 binary download crashes with a confusing error unrelated to the real cause
- Stack trace points into Node.js internals (`_http_client.js`) rather than
  application code
- Error message does not mention the URL or that a redirect was being followed
- Socket may remain open (resource leak) if the response body is not drained

## What Did Not Work

Leaving `get(headers.location)` unguarded:

```js
// BROKEN — passes undefined silently when headers.location is absent
if (statusCode === 301 || statusCode === 302) {
  return get(headers.location);
}
```

Node.js propagates `undefined` through the call stack silently. The failure
point is far removed from the missing header, making root cause diagnosis slow.

## Solution

### Before

```js
function fetchBuf(url) {
  return new Promise((resolve, reject) => {
    const get = (u) =>
      https.get(u, (res) => {
        const { statusCode, headers } = res;
        if (statusCode === 301 || statusCode === 302) {
          return get(headers.location);      // ← undefined when header absent
        }
        if (statusCode !== 200) {
          return reject(new Error(`HTTP ${statusCode}: ${u}`));
        }
        const chunks = [];
        res.on('data', (c) => chunks.push(c));
        res.on('end', () => resolve(Buffer.concat(chunks)));
        res.on('error', reject);
      }).on('error', reject);
    get(url);
  });
}
```

### After

```js
function fetchBuf(url) {
  return new Promise((resolve, reject) => {
    const get = (u) =>
      https.get(u, (res) => {
        const { statusCode, headers } = res;
        if (statusCode === 301 || statusCode === 302) {
          res.resume();                       // drain body — prevents socket leak
          if (!headers.location) {
            return reject(
              new Error(`Redirect (${statusCode}) without Location header: ${u}`)
            );
          }
          return get(headers.location);
        }
        if (statusCode !== 200) {
          res.resume();                       // drain body on error responses too
          return reject(new Error(`HTTP ${statusCode}: ${u}`));
        }
        const chunks = [];
        res.on('data', (c) => chunks.push(c));
        res.on('end', () => resolve(Buffer.concat(chunks)));
        res.on('error', reject);
      }).on('error', reject);
    get(url);
  });
}
```

**Two changes applied:**

1. `res.resume()` on redirect and error branches — drains the response body so
   Node.js can reuse the socket rather than keeping it open indefinitely
2. Guard `!headers.location` before recursive call — rejects the promise with a
   descriptive message that names the status code and URL

## Why This Works

Node.js `http`/`https` modules do not validate URL parameters at the API
boundary. Passing `undefined` to `https.get()` succeeds the call but fails
deep in the URL parsing path with an error that has no context about the
original request or the redirect chain.

The guard intercepts the invalid condition at the earliest point — immediately
after the response is received — and produces an error message with full context.

`res.resume()` is necessary because Node.js keeps the underlying TCP socket
open until the response body is fully consumed. For redirect and error responses
where the body is not read, calling `res.resume()` signals that the body should
be discarded, freeing the socket.

## Prevention

- Always guard `headers.location` before following a redirect in Node.js:

  ```js
  if (!headers.location) {
    return reject(new Error(`Redirect without Location header from ${url}`));
  }
  ```

- Always call `res.resume()` on response branches that do not read the body
  (3xx, 4xx, 5xx, or any early-return path)

- Add a maximum redirect depth counter to prevent infinite redirect loops:

  ```js
  const MAX_REDIRECTS = 5;
  // pass redirectCount through the closure, increment on each redirect
  ```

- Test malformed redirect scenarios explicitly:

  ```js
  // Mock: return 302 with no Location header
  // Assert: error message contains the status code and URL
  // Assert: promise rejects (does not hang)
  ```

## Related Solutions

- `docs/compound/best-practices/npm-hybrid-go-binary-resolver-2026-04-28.md` —
  the tier3 download context where this fix was applied
