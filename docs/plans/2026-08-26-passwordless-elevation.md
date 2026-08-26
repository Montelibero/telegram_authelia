# Passwordless Elevation Implementation Plan

**Goal:** Remove mandatory email OTP and password follow-up from passkey and Telegram one-factor flows when explicitly configured.

**Architecture:** Add an opt-in elevation policy flag, enforce it at middleware and OTP endpoints, use generic elevation for Telegram linking, and correct portal navigation for targetless possession/external sessions.

**Tech Stack:** Go, React/TypeScript, Vitest, testify.

---

1. Add failing Go tests for fresh passwordless elevation and disabled OTP generation.
2. Add a failing React test for targetless one-factor sessions without knowledge.
3. Add the configuration field and known key.
4. Implement recent-auth elevation and disable OTP endpoints under the flag.
5. Move Telegram link/unlink to generic elevation middleware.
6. Correct login portal navigation while preserving protected two-factor targets.
7. Run focused tests, then full Go/web verification and image smoke tests.
