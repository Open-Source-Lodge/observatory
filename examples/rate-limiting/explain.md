# Rate limit calls to external services

## Why

An external service has a quota. A loop that calls the service without a
limit hits the quota, and then each call fails until the quota resets. Some
services bill by the call, so a loop without a limit also costs money.

A limiter in the client puts the limit in one place. A caller cannot forget
it.

## What the rule covers

- Each new client for an HTTP API, a message queue or a third-party SDK.
- Each new call site that calls an external service in a loop or from a
  worker.
- Each removal of a limiter from an existing client.

## What the rule does not cover

- Calls to services that this repository owns, such as the internal database.
- Test code with a mock transport.
- A one-time call at start-up, such as a health check.
