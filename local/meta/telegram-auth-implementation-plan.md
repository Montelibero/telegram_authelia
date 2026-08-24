# Telegram Authentication and SQL Users Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an overlay-owned SQL user backend, safe YAML migration, and Telegram OIDC login while preserving Authelia sessions, Forward Auth, ACL, and second-factor behavior.

**Architecture:** Implement overlay tables and queries through the existing `storage.SQLProvider` connection, expose them to a new `authentication.SQLUserProvider` through a narrow interface, and select that provider in `middlewares.NewProviders` only when the new backend is configured. Telegram resolves a stable numeric provider identity to the same local user and delegates session creation to the existing first-factor flow.

**Tech Stack:** Go, Cobra, sqlx, SQLite integration tests, React/TypeScript, Vite, Vitest, Telegram OIDC Authorization Code Flow with PKCE.

---

## Working rules

- Read the approved design with `git show local/meta:local/meta/telegram-auth-design.md` before implementation.
- Create each topic branch from `release-base`, never from `master` or `deploy`.
- Use `codespaces` before each non-trivial slice and rebuild the belief map after adding modules.
- Follow TDD: failing focused test, minimal implementation, passing focused test, broader verification, commit.
- Do not push, publish, rebuild `deploy`, or deploy without explicit user approval.
- Integrate completed topic branches into `local/auth-overlay` only after review.

## M0: SQL users and migration

### Task 1: Add SQL authentication configuration

**Branch:** `feat/sql-user-provider`

**Files:**

- Modify: `internal/configuration/schema/authentication.go`
- Modify: `internal/configuration/schema/authentication_test.go`
- Modify: `internal/configuration/validator/authentication.go`
- Modify: `internal/configuration/validator/authentication_test.go`
- Modify: `internal/configuration/defaults.go`
- Modify: `internal/configuration/validator/configuration_test.go`

**Steps:**

1. Add failing schema tests for an `authentication_backend.sql` option with configurable `generated_email_domain`.
2. Add validator cases proving exactly one of file, LDAP, or SQL is selected and that SQL requires an existing SQL storage backend plus a valid generated email domain.
3. Run `go test ./internal/configuration/schema ./internal/configuration/validator` and confirm the new tests fail.
4. Add `AuthenticationBackendSQL` and the minimal validation/default logic. Do not duplicate storage connection settings under authentication.
5. Rerun the focused tests and confirm they pass.
6. Run `go test ./internal/configuration/...`.
7. Commit with `feat(config): add SQL authentication backend`.

### Task 2: Add overlay schema migrations

**Files:**

- Create: `internal/storage/mtl_migrations.go`
- Create: `internal/storage/mtl_migrations_test.go`
- Create: `internal/storage/migrations/mtl/0001_users.up.sql`
- Create: `internal/storage/migrations/mtl/0001_users.down.sql`
- Create: `internal/model/mtl_user.go`
- Create: `internal/model/mtl_user_test.go`

**Steps:**

1. Write SQLite integration tests asserting creation of `mtl_schema_migrations`, `mtl_users`, `mtl_user_emails`, `mtl_user_identities`, `mtl_groups`, `mtl_group_memberships`, and `mtl_audit_events`.
2. Assert unique local username, unique normalized email, unique `(provider, provider_user_id)`, one primary email per user, and valid status values.
3. Run `go test ./internal/storage -run MTL -count=1` and confirm failure.
4. Implement an embedded, independently versioned migration runner using the existing `SQLXConnection` and transaction primitives.
5. Implement migration 1 for SQLite-compatible SQL; keep query construction compatible with existing rebinding so PostgreSQL/MySQL are not artificially blocked.
6. Rerun focused tests, including applying migrations twice and rolling back a failed transaction.
7. Run `go test ./internal/storage ./internal/model`.
8. Commit with `feat(storage): add user overlay schema`.

### Task 3: Add the narrow SQL user store

**Files:**

- Create: `internal/storage/mtl_users.go`
- Create: `internal/storage/mtl_users_test.go`
- Modify: `internal/storage/sql_provider.go`
- Modify: `internal/storage/sql_provider_test.go`

**Steps:**

1. Define focused store operations for user lookup, password update, primary email, groups, status, and transactional import.
2. Write failing SQL mock tests and SQLite integration tests for active, disabled, missing, Telegram-only, and grouped users.
3. Include optimistic version checks and controlled unique-conflict mapping.
4. Run `go test ./internal/storage -run 'MTLUser|SQLProvider' -count=1` and confirm failure.
5. Implement methods on `SQLProvider` without exposing its private database handle and without adding user methods to the large shared `storage.Provider` interface.
6. Rerun focused and full storage tests.
7. Commit with `feat(storage): add SQL user store`.

### Task 4: Implement `SQLUserProvider`

**Files:**

- Create: `internal/authentication/sql_user_provider.go`
- Create: `internal/authentication/sql_user_provider_test.go`
- Modify: `internal/authentication/user_provider.go`
- Modify: `internal/authentication/types.go`

**Steps:**

1. Define a narrow `SQLUserStore` interface in `authentication` containing only the operations required by `UserProvider`.
2. Write a shared provider contract test covering `CheckUserPassword`, `GetDetails`, `GetDetailsExtended`, `UpdatePassword`, `ChangePassword`, disabled users, null passwords, primary email, and groups.
3. Reuse the existing password digest parsing and verification behavior; do not invent a new hashing format.
4. Run `go test ./internal/authentication -run SQLUserProvider -count=1` and confirm failure.
5. Implement `SQLUserProvider` and make `Close` a no-op because the storage provider owns the connection lifecycle.
6. Rerun all authentication tests.
7. Commit with `feat(authentication): add SQL user provider`.

### Task 5: Provision the SQL provider without changing other backends

**Files:**

- Modify: `internal/middlewares/util.go`
- Modify: `internal/middlewares/util_test.go`
- Modify: `internal/middlewares/startup.go`
- Modify: `internal/middlewares/startup_test.go`

**Steps:**

1. Add failing tests that file and LDAP construction remain unchanged and SQL authentication receives the already-created storage provider.
2. Add a negative test for a non-SQL or incompatible storage provider.
3. Change `NewAuthenticationProvider` to accept the provisioned storage provider and select `authentication.NewSQLUserProvider` only for configured SQL authentication.
4. Run `go test ./internal/middlewares -run 'AuthenticationProvider|Providers|Startup' -count=1`.
5. Run `go test ./internal/middlewares ./internal/authentication ./internal/storage`.
6. Commit with `feat(authentication): provision SQL users from storage`.

### Task 6: Add idempotent YAML import with dry-run

**Branch:** `feat/yaml-user-migration`

**Files:**

- Create: `internal/authentication/sql_user_import.go`
- Create: `internal/authentication/sql_user_import_test.go`
- Modify: `internal/commands/storage.go`
- Modify: `internal/commands/storage_run.go`
- Modify: `internal/commands/storage_run_test.go`
- Modify: `internal/commands/const.go`
- Add fixture: `internal/commands/testdata/users_database.yml`

**Steps:**

1. Add failing importer tests for usernames, display names, hashes, multiple emails, no email fallback, groups, duplicate input, existing SQL records, and rerun idempotency.
2. Add a failing CLI test for `authelia storage user import --from PATH --dry-run` proving no rows are written and a deterministic report is emitted.
3. Implement parsing by reusing the file user database model instead of introducing a second YAML schema.
4. Implement conflict collection before mutation and one transaction for executable import.
5. Ensure hashes are copied byte-for-byte and never logged.
6. Run `go test ./internal/authentication ./internal/commands -run 'Import|StorageUser' -count=1`.
7. Run the CLI twice against a temporary SQLite database and compare row counts.
8. Commit with `feat(commands): import file users into SQL`.

### Task 7: Prove password and Forward Auth parity

**Files:**

- Modify: `internal/handlers/handler_firstfactor_password_test.go`
- Modify: `internal/handlers/handler_authz_test.go`
- Modify: `internal/handlers/handler_authz_impl_forwardauth_test.go`
- Modify: `internal/middlewares/startup_test.go`

**Steps:**

1. Add equivalent file and SQL backend test cases for successful password login, disabled user rejection, `Remote-User`, `Remote-Email`, `Remote-Groups`, and `Remote-Name`.
2. Confirm the new SQL cases fail before wiring is complete.
3. Make only the minimal integration corrections required for parity.
4. Run focused handler tests and then `go test ./internal/handlers ./internal/middlewares ./internal/authentication ./internal/storage`.
5. Run `go test ./internal/...` before merging the M0 branches into `local/auth-overlay`.
6. Commit with `test(auth): verify SQL Forward Auth parity`.

### Task 8: Document and rehearse M0 cutover

**Files:**

- Create: `local/meta/sql-user-cutover.md` on `local/meta`
- Modify: `local/meta/BRANCHES.md` on `local/meta`

**Steps:**

1. Document exact backup, dry-run, import, verification, startup, smoke-test, and rollback commands without embedding secrets.
2. Test the procedure against a disposable copy of the production-shaped SQLite database and YAML fixture.
3. Verify rollback with the upstream image and preserved YAML.
4. Record observed row counts and checksums in a private run log, not in repository documentation.
5. Commit the generic runbook on `local/meta` with `docs(auth): add SQL user cutover runbook`.

## M1: Telegram OIDC and account linking

### Task 9: Add Telegram configuration and validation

**Branch:** `feat/telegram-login`

**Files:**

- Create: `internal/configuration/schema/telegram.go`
- Create: `internal/configuration/schema/telegram_test.go`
- Create: `internal/configuration/validator/telegram.go`
- Create: `internal/configuration/validator/telegram_test.go`
- Modify: `internal/configuration/schema/configuration.go`
- Modify: `internal/configuration/validator/configuration.go`
- Modify: `internal/configuration/defaults.go`

**Steps:**

1. Add failing tests for enabled state, issuer, client ID, secret reference, exact public callback URL, and disabled defaults.
2. Require HTTPS for public callback URLs except explicit development/test fixtures.
3. Add configuration without logging or serializing the resolved client secret.
4. Run `go test ./internal/configuration/...`.
5. Commit with `feat(config): add Telegram login settings`.

### Task 10: Implement the Telegram OIDC client and mock issuer tests

**Files:**

- Create: `internal/telegram/client.go`
- Create: `internal/telegram/client_test.go`
- Create: `internal/telegram/state.go`
- Create: `internal/telegram/state_test.go`
- Create: `internal/telegram/testdata_test.go`

**Steps:**

1. Build a local mock issuer with authorization, token, discovery, and JWKS endpoints.
2. Write failing tests for state, nonce, PKCE S256, issuer, audience, expiry, invalid signature, missing claims, reused state, and identity extraction.
3. Implement authorization URL creation and code exchange using a maintained OIDC/OAuth library already compatible with the repository dependency policy.
4. Keep state short-lived, single-use, and bound to the original flow and return URL.
5. Run `go test ./internal/telegram -count=1` and `go test -race ./internal/telegram`.
6. Commit with `feat(telegram): add verified OIDC client`.

### Task 11: Add Telegram identity storage and CLI recovery

**Branch:** `feat/telegram-account-linking`

**Files:**

- Modify: `internal/storage/mtl_users.go`
- Modify: `internal/storage/mtl_users_test.go`
- Modify: `internal/commands/storage.go`
- Modify: `internal/commands/storage_run.go`
- Modify: `internal/commands/storage_run_test.go`

**Steps:**

1. Add failing tests for link, lookup, unlink, duplicate provider ID, unknown user, and refusal to overwrite an existing link.
2. Implement transactional identity operations and audit events.
3. Add `storage user identity link`, `unlink`, and `show` commands with explicit provider and provider user ID arguments.
4. Ensure ordinary output does not reveal secrets and destructive unlink requires an exact username/provider match.
5. Run focused storage and command tests.
6. Commit with `feat(commands): manage external user identities`.

### Task 12: Add Telegram login handlers and session handoff

**Files:**

- Create: `internal/handlers/handler_telegram_login.go`
- Create: `internal/handlers/handler_telegram_login_test.go`
- Modify: `internal/server/handlers.go`
- Modify: `internal/server/handlers_test.go`
- Modify: `internal/middlewares/types.go`
- Modify: `internal/middlewares/util.go`

**Steps:**

1. Add handler tests for start, callback, known active user, unknown identity, disabled user, replay, invalid state, and safe return URL handling.
2. Assert successful Telegram login uses the same session regeneration and one-factor state as password first-factor login.
3. Assert it never marks the session as two-factor.
4. Register `GET /api/telegram/login` and `GET /api/telegram/callback` with no-store security headers and appropriate rate limiting.
5. Run focused handler/server tests and then `go test ./internal/handlers ./internal/server ./internal/middlewares ./internal/telegram`.
6. Commit with `feat(auth): add Telegram first-factor login`.

### Task 13: Add elevated self-service linking

**Files:**

- Create: `internal/handlers/handler_telegram_link.go`
- Create: `internal/handlers/handler_telegram_link_test.go`
- Modify: `internal/server/handlers.go`
- Modify: `internal/server/handlers_test.go`

**Steps:**

1. Add failing tests requiring an authenticated user and fresh password elevation.
2. Test start, callback, collision, unlink, expired elevation, CSRF/state mismatch, and audit creation.
3. Register link endpoints behind existing elevation middleware.
4. Keep the Telegram provider identity unique and bind the callback to the authenticated local username.
5. Run handler, session elevation, and storage tests.
6. Commit with `feat(auth): add Telegram account linking`.

### Task 14: Add Telegram login and linking UI

**Files:**

- Create: `web/src/services/Telegram.ts`
- Create: `web/src/services/Telegram.test.ts`
- Create: `web/src/components/TelegramLoginButton.tsx`
- Create: `web/src/components/TelegramLoginButton.test.tsx`
- Modify: `web/src/services/Api.ts`
- Modify: `web/src/views/LoginPortal/FirstFactor/FirstFactorForm.tsx`
- Modify: `web/src/views/LoginPortal/FirstFactor/FirstFactorForm.test.tsx`
- Modify: `web/src/views/LoginPortal/LoginPortal.tsx`
- Modify: `web/src/views/LoginPortal/LoginPortal.test.tsx`

**Steps:**

1. Add failing component tests for enabled/disabled configuration, safe redirect, loading, unlinked, disabled, expired, and generic errors.
2. Add a primary Telegram button while keeping the password form and passkey behavior intact.
3. Add the link action to the authenticated settings surface only after backend linking tests pass.
4. Run `pnpm -C web test --run FirstFactorForm Telegram LoginPortal` using the repository-supported filter syntax.
5. Run `pnpm -C web lint` and `pnpm -C web test`.
6. Commit with `feat(web): add Telegram authentication UI`.

### Task 15: Verify M1 end to end

**Files:**

- Add or modify the smallest relevant files under `internal/suites/` only if the existing suite harness can model the mock issuer without production secrets.
- Update: `local/meta/BRANCHES.md` on `local/meta` after integration.

**Steps:**

1. Run all new Go tests with `-race` for the Telegram and SQL user packages.
2. Run `go test ./internal/...`.
3. Run frontend lint and tests.
4. Build the Go binary and frontend using the repository's existing build path.
5. Start a disposable SQLite-backed instance with the mock issuer and verify password login, Telegram login, Forward Auth headers, two-factor policy preservation, disabled-user rejection, and account linking.
6. Perform a manual real-Telegram smoke test only after local and CI verification pass.
7. Integrate the reviewed topic branches into `local/auth-overlay`, update the belief map, inspect architecture violations, and request code review.
8. Do not rebuild or push `deploy` until explicit approval.

## Deferred plans

Create separate implementation plans after M1 is stable for:

- pending registration and approval;
- Admin Users API/UI;
- immediate global session revocation implementation if M0/M1 reveals an upstream limitation;
- applications and permissions matrix;
- password self-service;
- GitHub Actions `local/ci-deploy` overlay;
- Telegram Bot Login.
