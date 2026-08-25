# Password Management and Self-Service Implementation Plan

> Execute each production change with @superpowers:test-driven-development and verify completion with @superpowers:verification-before-completion. Use @codespaces before expanding a module boundary.

**Goal:** Complete M5 password lifecycle and user self-service while preserving existing Authelia session, policy, Telegram OIDC, and Forward Auth contracts.

**Architecture:** Add transactional overlay storage operations and narrow authenticated handlers. Reuse the existing Telegram OIDC state machinery with a new purpose instead of inventing a second verifier. Extend the existing Settings Security page and typed web services.

**Tech Stack:** Go, SQL/SQLite through Authelia SQL abstractions, fasthttp, React, TypeScript, MUI, Vitest.

---

### Task 45: Record the M5 design and executable plan

**Files:**
- Create: `docs/plans/2026-08-25-password-self-service-design.md`
- Create: `docs/plans/2026-08-25-password-self-service.md`

1. Record the approved password, Telegram proof, profile, session, and email boundaries.
2. Review paths and task dependencies against the Codespaces belief map.
3. Run `git diff --check` and commit the documents.

### Task 46: Add transactional password lifecycle storage

**Files:**
- Modify: `internal/model/mtl_user.go`
- Modify: `internal/storage/types.go`
- Modify: `internal/storage/mtl_users.go`
- Test: `internal/storage/mtl_users_test.go`

1. Write failing SQLite tests for setting, changing, and clearing a password with optimistic concurrency, session-epoch increment, audit actor/event, Telegram-link requirement, and last password-admin protection.
2. Run the focused storage test and confirm the expected RED failures.
3. Implement the minimal transactional storage methods without exposing password hashes.
4. Run focused storage tests with `-race` and commit.

### Task 47: Connect the SQL provider and preserve the current session

**Files:**
- Modify: `internal/authentication/sql_user_provider.go`
- Modify: `internal/authentication/types.go`
- Test: `internal/authentication/sql_user_provider_test.go`
- Modify: `internal/handlers/handler_change_password.go`
- Test: `internal/handlers/handler_change_password_test.go`

1. Write failing tests for password hashing, old-password verification, epoch rotation, audit attribution, and current-session epoch refresh.
2. Verify RED, implement the smallest provider/handler extension, then verify GREEN.
3. Ensure non-SQL user providers retain their existing behavior.
4. Commit the provider and standard password-change path.

### Task 48: Add fresh Telegram proof for first password setup

**Files:**
- Modify: `internal/telegram/state.go`
- Modify: `internal/telegram/provider.go`
- Modify: `internal/handlers/handler_telegram_login.go`
- Create: `internal/handlers/handler_self_service_password.go`
- Test: `internal/telegram/provider_test.go`
- Test: `internal/handlers/handler_self_service_password_test.go`
- Modify: `internal/server/handlers.go`

1. Write failing tests for the `password_setup` purpose, session/user binding, exact linked-identity match, expiry, replay, callback cookie, and unauthenticated rejection.
2. Verify RED before implementation.
3. Reuse state, nonce, PKCE, callback, and TTL machinery; issue only a single-use server-side proof bound to the current session user.
4. Add start, callback, and completion endpoints with password-policy validation and controlled errors.
5. Run focused race tests and commit.

### Task 49: Add password removal and display-name self-service

**Files:**
- Modify: `internal/handlers/handler_self_service_password.go`
- Create: `internal/handlers/handler_self_service_profile.go`
- Test: `internal/handlers/handler_self_service_password_test.go`
- Test: `internal/handlers/handler_self_service_profile_test.go`
- Modify: `internal/server/handlers.go`
- Modify: `internal/model/mtl_admin.go`

1. Write failing handler tests for current-password verification, linked-Telegram requirement, last password-admin conflict, display-name optimistic conflict, session refresh, audit, authentication, and CSRF routing.
2. Verify RED, implement minimal handlers and DTOs, then verify GREEN.
3. Keep username and email immutable through these endpoints.
4. Commit the self-service API.

### Task 50: Build the web self-service experience

**Files:**
- Modify: `web/src/models/UserInfo.ts`
- Modify: `web/src/services/UserInfo.ts`
- Create: `web/src/services/SelfService.ts`
- Test: `web/src/services/SelfService.test.ts`
- Modify: `web/src/views/Settings/Security/SecurityView.tsx`
- Modify: `web/src/views/Settings/Security/ChangePasswordDialog.tsx`
- Create: `web/src/views/Settings/Security/SetPasswordDialog.tsx`
- Create: `web/src/views/Settings/Security/DisablePasswordDialog.tsx`
- Create: `web/src/views/Settings/Security/EditProfileDialog.tsx`
- Test: `web/src/views/Settings/Security/SecurityView.test.tsx`
- Test: `web/src/views/Settings/Security/SetPasswordDialog.test.tsx`
- Test: `web/src/views/Settings/Security/DisablePasswordDialog.test.tsx`
- Test: `web/src/views/Settings/Security/EditProfileDialog.test.tsx`
- Modify: `internal/server/locales/en/settings.json`
- Modify: `internal/server/locales/ru/settings.json`
- Modify: `internal/server/locales_admin_test.go`

1. Write failing service and component tests for password-enabled/disabled states, Telegram redirect and proof completion, disable confirmation, profile editing, conflicts, and safe errors.
2. Verify RED before adding UI code.
3. Implement typed services and accessible MUI dialogs on the existing Security page.
4. Add English and Russian locale coverage, then run Vitest, ESLint, and TypeScript checks.
5. Commit the web experience.

### Task 51: Integration and security audit

**Files:**
- Modify tests or implementation only for concrete findings.

1. Run affected Go packages with `-race` as UID 1000 in Docker using a fresh retained Go cache.
2. Run the full web test suite, ESLint, TypeScript, and production build; restore generated tracked assets afterward.
3. Verify password secrets and Telegram proof material do not appear in logs or response DTOs.
4. Exercise SQLite setup, change, remove, display-name update, session rotation, and recovery-link compatibility.
5. Request independent code review and fix every Critical or Important finding through TDD.

### Task 52: Integrate the milestone

**Files:**
- Modify on `local/meta`: `local/meta/BRANCHES.md`

1. Merge `feat/password-self-service` into `local/auth-overlay` with an explicit merge commit.
2. Mark M5 complete in the branch registry.
3. Re-run key verification on the merge commit and prove topic-branch ancestry.
4. Do not push, tag, delete branches, or modify the server.
