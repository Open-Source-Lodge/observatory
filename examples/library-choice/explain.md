# Use httpx for HTTP calls

## Why

One HTTP library gives one place for timeouts, retries and headers. With two
libraries, a fix in one place does not apply to the other.

We chose `httpx` because it has the same API for sync and async code.

## What the rule covers

- Each import of `requests`, `urllib.request` or `aiohttp` in the source.
- Each new HTTP call in the source.

## What the rule does not cover

- Test code that uses `httpx.MockTransport`.
- Vendored third-party code under `vendor/`.
