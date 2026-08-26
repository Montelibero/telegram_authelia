# Activity Logging Design

## Goal

Make production activity understandable from the default `info` log level without exposing credentials or flooding the log with static assets and health checks.

## Request logging

The outer HTTP middleware emits one structured completion event for every meaningful request. The event includes method, normalized path, response status, elapsed time, remote IP, and the redirect destination when the response is a redirect. It does not include request or response bodies, cookies, authorization headers, OAuth codes, state values, or raw query strings.

Requests for health endpoints, metrics, static assets, the favicon, and manifest files are excluded. These endpoints create repetitive infrastructure traffic and do not describe user behavior. Unknown paths, rejected redirects, authentication callbacks, settings pages, and all API requests remain visible.

## Domain events

Request completion logs show that an action happened. Domain logs explain the result. Authentication handlers log the selected method and success or rejection. Telegram login logs the signed Telegram profile ID, provider username, registration status, and resolved local username. Administrator mutations continue to write durable rows to `mtl_audit_events`; successful audit writes also emit a structured application log containing actor ID, event type, target type, and target ID.

## Levels and privacy

Normal activity is `info`, rejected or suspicious input is `warn`, and operational failures are `error`. The feature is enabled by default and uses the existing global log level; no separate configuration switch is introduced. Sensitive request data is never logged. Numeric Telegram IDs and local usernames are treated as operational identity metadata and may appear in the administrator-controlled log.

## Verification

Unit tests cover filtering, request completion fields, redirects, and audit event emission. Existing Telegram, handler, middleware, and storage tests must remain green. The final amd64 deployment image must pass the existing smoke test.
