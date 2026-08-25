# Admin Users Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a secure, complete Web administration workflow for users, pending Telegram registrations, email addresses, identities, groups, memberships, and password setup.

**Architecture:** Extend the overlay SQL model behind narrow admin service interfaces, expose authenticated `/api/admin/*` handlers using existing Authelia middleware and password elevation, and add a lazy-loaded `/admin` portal route. Use optimistic versions, transactions, audit events, and a per-user session epoch for immediate logical revocation.

**Tech Stack:** Go 1.26, Cobra, SQL/SQLite overlay migrations, fasthttp handlers, Authelia sessions and one-time tokens, React 19, TypeScript, MUI, Vitest, Testing Library.

---

## Working rules

- Branch `feat/admin-users` from reviewed `local/auth-overlay`.
- Query Codespaces before every non-trivial slice and preserve `.belief_map.sexp` locally.
- Use TDD: failing focused test, minimal implementation, focused pass, then package tests.
- Run Go through `golang:1.26.3-bookworm` with a writable cache bind and repository mounted read-only for tests.
- Keep code, docs, UI strings, commit messages, and security warnings professional.
- Do not push, tag, rebuild `deploy`, or delete topic branches.

### Task 27: Extend admin storage model and session epoch

**Files:**
- Create: `internal/storage/migrations/mtl/0003_admin_users.up.sql`
- Create: `internal/storage/migrations/mtl/0003_admin_users.down.sql`
- Modify: `internal/model/mtl_user.go`
- Modify: `internal/storage/mtl_migrations_test.go`
- Test: `internal/storage/mtl_admin_test.go`

1. Write migration tests expecting schema version 3, user session epoch, and group optimistic version/timestamps.
2. Run focused migration tests and confirm failure at version 2.
3. Add `session_epoch` to users and `version`, `created_at`, `updated_at` to groups using SQLite-compatible migration steps.
4. Add admin DTOs for user summaries/details, email, identity, group, and mutation requests without password hashes.
5. Test migration apply/down, constraints, and model status/version validation.
6. Run `go test ./internal/model ./internal/storage -run 'MTL.*Admin|Migration' -count=1`.
7. Commit `feat(storage): extend admin user schema`.

### Task 28: Implement transactional users, email, and identity administration

**Files:**
- Create: `internal/storage/mtl_admin_users.go`
- Test: `internal/storage/mtl_admin_users_test.go`
- Modify: `internal/storage/errors.go`

1. Write failing SQLite tests for list/detail/create/update/status, email add/remove/primary, and Telegram unlink.
2. Cover immutable username, unique primary email, expected-version conflicts, verified admin-created email, and rollback.
3. Implement narrow transactional store methods; never return password hashes.
4. Increment `session_epoch` in the same transaction when disabling a user.
5. Emit actor-linked audit events for every mutation.
6. Run focused tests with `-race`, then full `./internal/storage`.
7. Commit `feat(storage): add admin user workflow`.

### Task 29: Implement group administration and bootstrap CLI

**Files:**
- Create: `internal/storage/mtl_admin_groups.go`
- Test: `internal/storage/mtl_admin_groups_test.go`
- Create: `internal/commands/storage_group.go`
- Create: `internal/commands/storage_group_test.go`
- Modify: `internal/commands/storage.go`

1. Write failing tests for group list/show/create/rename/delete and membership add/remove.
2. Assert rename/delete reports affected users and does not impose `admins`/`app:*` naming restrictions.
3. Implement transactional group methods with optimistic versions and audit events.
4. Add discoverable Cobra commands `storage group list/show/create/rename/delete/add-user/remove-user`, each with examples and help.
5. Prove first-admin bootstrap and recovery lifecycle against disposable SQLite.
6. Run commands/storage tests and real `--help` output.
7. Commit `feat(cli): add group recovery commands`.

### Task 30: Add Admin API authorization and password elevation

**Files:**
- Create: `internal/middlewares/admin.go`
- Create: `internal/middlewares/admin_test.go`
- Create: `internal/handlers/handler_admin.go`
- Modify: `internal/server/handlers.go`
- Modify: `internal/server/routes.go`

1. Write handler/middleware tests for anonymous, authenticated non-admin, Telegram-authenticated admin read, passwordless admin mutation, stale/non-elevated mutation, and elevated admin mutation.
2. Reuse normal session user details to check `admins` on every request.
3. Compose existing fresh password elevation and CSRF middleware for all mutations; do not restart the login flow.
4. Return safe JSON categories with `401`, `403`, and `409` semantics.
5. Test `/auth` base-path routing and method restrictions.
6. Run middleware/handler/server tests.
7. Commit `feat(api): protect admin routes`.

### Task 31: Expose Users and Groups Admin API

**Files:**
- Create: `internal/handlers/handler_admin_users.go`
- Create: `internal/handlers/handler_admin_users_test.go`
- Create: `internal/handlers/handler_admin_groups.go`
- Create: `internal/handlers/handler_admin_groups_test.go`
- Modify: `internal/middlewares/types.go`
- Modify: `internal/middlewares/util.go`
- Modify: `internal/server/routes.go`

1. Write failing contract tests for all agreed user, email, identity, group, and membership endpoints.
2. Add request decoding with expected versions and self-action typed-username confirmation.
3. Wire narrow admin store interfaces through providers.
4. Ensure responses omit hashes, secrets, tokens, and unrelated provider data.
5. Verify simultaneous edits yield exactly one success and one `409`.
6. Run focused tests and full handler/middleware packages.
7. Commit `feat(api): manage users and groups`.

### Task 32: Extend registration approval for groups and Admin API

**Files:**
- Modify: `internal/model/mtl_registration.go`
- Modify: `internal/storage/mtl_registrations.go`
- Modify: `internal/storage/mtl_registrations_test.go`
- Create: `internal/handlers/handler_admin_registrations.go`
- Create: `internal/handlers/handler_admin_registrations_test.go`

1. Write failing tests for status tabs, editable approval fields, initial groups, reject, stale version, and transaction rollback.
2. Extend approval DTO with display name and group names.
3. Atomically create memberships during approval with no automatic defaults.
4. Expose list/show/approve/reject admin endpoints.
5. Preserve CLI behavior and backward-compatible defaults.
6. Run registration storage/CLI/handler tests with race coverage.
7. Commit `feat(api): administer pending registrations`.

### Task 33: Implement immediate logical session revocation

**Files:**
- Modify: `internal/session/user_session.go`
- Modify: `internal/authentication/sql_user_provider.go`
- Modify: `internal/handlers/handler_state.go`
- Modify: `internal/middlewares/require_auth.go`
- Test: corresponding session, authentication, handler, and authorization tests

1. Trace current session serialization and periodic user-detail refresh with Codespaces before editing.
2. Write failing tests: login captures epoch; disable increments epoch; next authenticated request destroys the stale session; enable does not restore it.
3. Add the smallest backward-compatible epoch field to the overlay-backed session/user detail flow.
4. Keep file/LDAP providers unaffected and old serialized sessions readable.
5. Verify Forward Auth immediately rejects disabled/stale sessions.
6. Run session/authentication/authorization/handler tests.
7. Commit `feat(session): revoke disabled user sessions`.

### Task 34: Add one-time password setup links

**Files:**
- Create: `internal/handlers/handler_admin_password_setup.go`
- Create: `internal/handlers/handler_admin_password_setup_test.go`
- Modify: `internal/handlers/handler_reset_password.go`
- Modify: `internal/model/one_time_code.go`
- Modify: `web/src/views/ResetPassword/ResetPasswordStep2.tsx` only if setup semantics require a safe label change

1. Write failing tests for admin authorization/elevation, opaque URL creation, expiry, single use, replay rejection, and SQL password update.
2. Reuse the existing reset completion route/token storage; do not create a second password-setting implementation.
3. Return the setup URL once and exclude it from logs/audit payloads.
4. Keep notifier support intact without adding mail configuration or delivery requirements.
5. Run reset-password, token, and SQL authentication tests.
6. Commit `feat(auth): add admin password setup links`.

### Task 35: Add Admin shell and Users UI

**Files:**
- Modify: `web/src/App.tsx`
- Modify: `web/src/constants/Routes.ts`
- Create: `web/src/views/Admin/AdminRouter.tsx`
- Create: `web/src/layouts/AdminLayout.tsx`
- Create: `web/src/views/Admin/UsersView.tsx`
- Create: `web/src/views/Admin/UserDialog.tsx`
- Create: `web/src/services/Admin.ts`
- Create: `web/src/models/Admin.ts`
- Test: matching `.test.tsx` and `.test.ts` files

1. Write failing tests for admin-only navigation, lazy route, complete list, local search, create/edit, emails, primary selection, memberships, status, identity warning, self-confirmation, conflict reload, and setup-link copy.
2. Implement typed service/model layer and lazy `/admin` route.
3. Reuse existing layouts, notifications, elevation dialog, theme tokens, and accessibility patterns.
4. Never show password hashes or stable provider IDs outside the detail view needed by admins.
5. Add English and Russian locale keys.
6. Run focused Vitest, TypeScript, Prettier, and theme tests.
7. Commit `feat(web): add admin users interface`.

### Task 36: Add Pending and Groups UI

**Files:**
- Create: `web/src/views/Admin/RegistrationsView.tsx`
- Create: `web/src/views/Admin/RegistrationDialog.tsx`
- Create: `web/src/views/Admin/GroupsView.tsx`
- Create: `web/src/views/Admin/GroupDialog.tsx`
- Modify: `web/src/views/Admin/AdminRouter.tsx`
- Modify: `web/src/services/Admin.ts`
- Test: matching frontend tests

1. Write failing tests for Pending/Approved/Rejected tabs, approval overrides/groups, reject, stale reload, group CRUD, affected-user warnings, and arbitrary names.
2. Implement responsive MUI tables/dialogs without a separate SPA or design system.
3. Preserve `/auth` base path and keyboard/screen-reader operation.
4. Add English and Russian strings.
5. Run focused and full frontend verification.
6. Commit `feat(web): administer registrations and groups`.

### Task 37: Verify, review, and integrate M3

**Files:**
- Modify: `local/meta/BRANCHES.md` on `local/meta`
- Update: `.belief_map.sexp` locally only

1. Run all affected Go packages and focused race tests with a fresh writable Go cache.
2. Run full frontend tests, TypeScript, formatting, and production build.
3. Run a disposable SQLite API/UI lifecycle: CLI bootstrap, web-created user, Telegram approval with groups, email primary change, setup link, disable/revocation, and emergency CLI recovery.
4. Rebuild/query Codespaces and inspect changed boundaries.
5. Request independent code review for the complete M3 range; fix every Critical/Important finding and re-review.
6. Merge reviewed `feat/admin-users` into `local/auth-overlay` with an explicit merge commit.
7. Re-run verification on the merge commit and mark M3 ready in `local/meta/BRANCHES.md`.
8. Do not push, tag, rebuild `deploy`, or delete the topic branch.

