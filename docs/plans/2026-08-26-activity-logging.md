# Activity Logging Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development to implement this plan task-by-task.

**Goal:** Emit useful user, authentication, redirect, and administrator activity at the default `info` log level without logging secrets or infrastructure noise.

**Architecture:** Upgrade the existing outer request middleware from two trace messages to one structured completion message with filtering. Keep durable administrator audit rows as the source of truth and mirror successful audit writes to the application log. Preserve explicit Telegram outcome events.

**Tech Stack:** Go, fasthttp, logrus, SQLite-backed storage, testify.

---

### Task 1: Meaningful request completion logs

**Files:**
- Modify: `internal/middlewares/log_request.go`
- Modify: `internal/middlewares/log_request_test.go`

1. Write failing tests proving that an API request emits one `info` event after the handler completes with method, path, status, duration, and redirect location.
2. Write failing table tests proving that health, metrics, static, favicon, and manifest paths do not emit activity events.
3. Run `go test ./internal/middlewares -run 'TestLogRequest' -count=1` in the pinned Go Docker image and confirm the tests fail for the missing behavior.
4. Implement path filtering and the single completion event without query strings, bodies, cookies, or headers.
5. Run the targeted middleware tests and then the full middleware package.
6. Commit as `feat(logging): record meaningful HTTP activity`.

### Task 2: Administrator audit event logs

**Files:**
- Modify: `internal/storage/mtl_admin_users.go`
- Modify: `internal/storage/mtl_admin_users_test.go`
- Modify: `internal/storage/mtl_admin_groups_test.go`

1. Write a failing test that captures the logger and proves a committed administrator mutation emits actor ID, event type, target type, and target ID at `info`.
2. Confirm rollback/error paths do not claim that the mutation succeeded.
3. Emit the log only after the audit insert succeeds; never include mutation request bodies or credentials.
4. Run the targeted tests and the full storage package.
5. Commit as `feat(logging): expose administrator audit events`.

### Task 3: Regression and deployment verification

**Files:**
- Modify only if a regression requires a scoped correction.

1. Run `go test ./internal/middlewares ./internal/handlers ./internal/telegram ./internal/storage -count=1` in Docker.
2. Run `git diff --check` and scan the diff for credentials.
3. Merge the topic into `local/auth-overlay` using the established overlay workflow.
4. Rebuild `deploy`, run production-package and workflow verification, build the amd64 image, and run its smoke test.
5. Push overlay normally and deploy with an exact `--force-with-lease` expectation.
6. Wait for GitHub Actions, pull `ghcr.io/montelibero/authelia:latest`, and verify its architecture, revision label, and digest.
