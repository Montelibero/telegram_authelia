# Telegram Pending Registration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Record unknown Telegram identities as pending registration requests and provide safe CLI approval/rejection without creating users before approval.

**Architecture:** Add an independently versioned MTL migration and narrow SQL registration store on the existing `storage.SQLProvider`. Telegram login upserts unknown identities through a registration service, while CLI approval performs user, email, identity, request-state, and audit writes in one transaction.

**Tech Stack:** Go, Cobra, sqlx, SQLite integration tests, Telegram OIDC, React/TypeScript, Vite, Vitest.

---

## Working rules

- Branch M2 from `local/auth-overlay` because it depends on the integrated M0/M1 overlay.
- Use `codespaces` before each non-trivial slice and rebuild `.belief_map.sexp` after new modules.
- Follow TDD for every task: failing focused test, minimal implementation, passing focused test, broader verification, commit.
- Do not push, tag, rebuild `deploy`, or deploy without explicit approval.
- Keep generated belief-map files locally excluded and do not delete them.

### Task 16: Add registration request schema and model

**Files:**

- Create: `internal/storage/migrations/mtl/0002_registration_requests.up.sql`
- Create: `internal/storage/migrations/mtl/0002_registration_requests.down.sql`
- Create: `internal/model/mtl_registration.go`
- Create: `internal/model/mtl_registration_test.go`
- Modify: `internal/storage/mtl_migrations_test.go`

**Steps:**

1. Write failing SQLite migration tests for `mtl_registration_requests`, unique `(provider, provider_user_id)`, valid statuses, optimistic version, nullable proposals, and timestamps.
2. Run `go test ./internal/storage ./internal/model -run 'MTL.*Registration' -count=1` and confirm failure.
3. Add model constants and request/approval DTOs without arbitrary length or format restrictions beyond SQL/protocol requirements.
4. Add migration 2 and verify applying MTL migrations twice is idempotent.
5. Run `go test ./internal/storage ./internal/model`.
6. Commit: `feat(storage): add pending registration schema`.

### Task 17: Add registration store and atomic approval

**Files:**

- Create: `internal/storage/mtl_registrations.go`
- Create: `internal/storage/mtl_registrations_test.go`
- Modify: `internal/storage/mtl_users.go`

**Steps:**

1. Write failing SQLite tests for unknown-identity upsert, repeated login update, rejected request preservation, list/show, missing request, and optimistic version conflicts.
2. Add failing approval tests proving one transaction creates an active passwordless user, verified primary email, Telegram identity, no groups, terminal request, and audit events.
3. Add collision and forced rollback tests proving no partial user is created.
4. Run `go test ./internal/storage -run 'MTLRegistration' -count=1` and confirm failure.
5. Implement a narrow store with parameterized/rebound queries and controlled domain errors.
6. Run focused tests, then `go test ./internal/storage ./internal/model`.
7. Commit: `feat(storage): add pending registration workflow`.

### Task 18: Capture unknown Telegram identities during login

**Files:**

- Create: `internal/telegram/registration.go`
- Create: `internal/telegram/registration_test.go`
- Modify: `internal/telegram/login.go`
- Modify: `internal/telegram/login_test.go`
- Modify: `internal/middlewares/util.go`
- Modify: `internal/handlers/handler_telegram_login.go`
- Modify: `internal/handlers/handler_telegram_login_test.go`

**Steps:**

1. Write failing service tests showing unknown verified identities create or update pending requests and never return a local user session.
2. Cover Telegram username present/absent, verified provider email, generated `<username>@generated-domain`, rejected replay, disabled linked users, and storage failure.
3. Write handler tests proving pending/rejected results redirect safely to the portal and do not authenticate the session.
4. Run `go test ./internal/telegram ./internal/handlers -run 'Telegram.*Registration|Telegram.*Pending' -count=1` and confirm failure.
5. Implement registration handoff through a narrow interface; retain stable numeric provider ID as the only identity key.
6. Run Telegram, handler, middleware, authentication, and storage tests.
7. Commit: `feat(auth): capture pending Telegram registrations`.

### Task 19: Add pending-result UI

**Files:**

- Modify: `web/src/views/LoginPortal/LoginPortal.tsx`
- Modify: `web/src/views/LoginPortal/LoginPortal.test.tsx`
- Modify: `web/src/components/TelegramLoginButton.tsx`
- Modify: `internal/server/locales/ru-RU/portal.json`

**Steps:**

1. Add failing component tests for pending and rejected callback results and generic safe fallback behavior.
2. Implement localized, non-enumerating messages without exposing a linked username or stable Telegram ID.
3. Run focused Vitest tests and confirm pass.
4. Run `corepack pnpm lint` and `corepack pnpm test -- --run` from `web/`.
5. Commit: `feat(web): show Telegram registration status`.

### Task 20: Add discoverable registration CLI

**Files:**

- Modify: `internal/commands/storage.go`
- Modify: `internal/commands/storage_run.go`
- Modify: `internal/commands/storage_run_test.go`
- Modify: `internal/commands/const.go`

**Steps:**

1. Add failing Cobra tests for `registration`, `list`, `show`, `approve`, and `reject`, including help text and examples at every command level.
2. Prove running `registration` without a subcommand prints its command list/help.
3. Add command tests for exact request ID, expected version, proposal defaults, required explicit username/email when missing, overrides, conflicts, and privacy-safe output.
4. Implement narrow CLI store adapters and deterministic tabular output.
5. Run `go test ./internal/commands -run 'StorageUserRegistration' -count=1`.
6. Run each command with `--help` against the built binary.
7. Commit: `feat(commands): manage pending registrations`.

### Task 21: Verify and integrate M2

**Files:**

- Modify: `local/meta/BRANCHES.md` on `local/meta` after review.

**Steps:**

1. Run `go test -race ./internal/telegram ./internal/storage ./internal/authentication ./internal/handlers ./internal/commands`.
2. Run `go test ./internal/...`; classify only reproducible upstream/infrastructure failures separately.
3. Run frontend lint, all frontend tests, and production build.
4. Build `./cmd/authelia` and exercise CLI help plus a disposable SQLite pending/approve/reject flow.
5. Verify an approved request logs in through Telegram and emits expected Forward Auth email/groups/name headers; groups must be empty until explicitly assigned.
6. Rebuild the belief map, check changed module boundaries, inspect the full diff for secrets, and request code review.
7. Fast-forward `local/auth-overlay` only after review reports no blocking findings.
8. Update `local/meta/BRANCHES.md` to M2 ready. Do not push or rebuild `deploy`.
