# Applications and Permissions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development to implement this plan task-by-task.

**Goal:** Let administrators grant application access from the web UI while keeping applications in YAML and authorization in standard Authelia groups and ACL rules.

**Architecture:** Add a small `applications` configuration section whose entries resolve to `app:<slug>` groups unless an explicit group is supplied. Reconcile missing configured groups into SQL without deleting historical groups. Expose read-only application metadata and optimistic membership mutations through the existing protected admin API. Build a permissions matrix over existing users and group memberships; the normal Authelia ACL engine remains unchanged.

**Tech Stack:** Go, SQLx, YAML/Koanf configuration, fasthttp, React 19, TypeScript, Material UI, Axios, Vitest.

---

### Task 38: Define applications configuration

**Files:**
- Modify: `internal/configuration/schema/configuration.go`
- Create: `internal/configuration/schema/applications.go`
- Create: `internal/configuration/validator/applications.go`
- Create: `internal/configuration/validator/applications_test.go`
- Modify: configuration key/default/example files where required

1. Write failing decode and validation tests for configured applications.
2. Support `slug`, `name`, `domain`, optional `group`, and `enabled`.
3. Derive an omitted group as `app:<slug>` without restricting otherwise valid group text.
4. Reject only ambiguity that makes the mapping unsafe: missing required values and duplicate slugs/groups.
5. Run focused configuration tests and commit.

### Task 39: Reconcile configured application groups

**Files:**
- Modify: SQL storage interfaces and startup wiring at the narrowest existing extension point
- Test: focused storage/startup tests

1. Write failing tests proving every enabled configured application has a SQL group.
2. Create missing groups idempotently and preserve existing memberships.
3. Never delete or rename groups merely because YAML changed.
4. Keep non-SQL authentication backends unaffected.
5. Run focused race tests and commit.

### Task 40: Add the protected applications API

**Files:**
- Create: `internal/handlers/handler_admin_applications.go`
- Create: `internal/handlers/handler_admin_applications_test.go`
- Modify: `internal/server/handlers.go`
- Modify: `internal/server/handlers_test.go`

1. Write failing tests for application metadata and user-by-application permission state.
2. Return only configured, enabled applications and safe user fields.
3. Reuse existing administrator read and fresh-password mutation middleware.
4. Preserve optimistic versions for every membership change.
5. Run handler and route race tests and commit.

### Task 41: Add permission mutation semantics

**Files:**
- Modify: admin application handler/storage helpers
- Test: handler and storage concurrency tests

1. Write failing tests for grant, revoke, duplicate requests, stale versions, and rollback.
2. Map grants and revocations directly to configured group membership.
3. Keep `admins` independent from application access and preserve default deny.
4. Return the refreshed user/application state after each mutation.
5. Run focused race tests and commit.

### Task 42: Add the typed applications web client and route

**Files:**
- Modify: `web/src/services/Api.ts`
- Modify: `web/src/services/Admin.ts`
- Modify: `web/src/services/Admin.test.ts`
- Modify: settings routes/navigation tests and components

1. Write failing API client and admin-only routing tests.
2. Add typed application and permission DTOs.
3. Add an administrator-only Permissions route and navigation item.
4. Run focused Vitest and TypeScript checks and commit.

### Task 43: Build the permissions matrix

**Files:**
- Create: `web/src/views/Settings/Admin/PermissionsView.tsx`
- Create: `web/src/views/Settings/Admin/PermissionsView.test.tsx`
- Modify: English settings locale

1. Write failing tests for loading, filtering, grant, revoke, password reauthentication, and conflict refresh.
2. Render a responsive user-by-application checkbox matrix.
3. Apply changes one at a time so every response supplies the next optimistic version.
4. Keep unchecked access as deny and never infer application access from `admins`.
5. Run focused tests, lint, type checking, and production build; commit.

### Task 44: Integration and security audit

1. Run all affected Go tests with the race detector.
2. Run the complete web test suite, lint, type checking, and production build.
3. Verify YAML decode, group reconciliation, non-admin access, fresh-password enforcement, stale versions, and default deny.
4. Review the milestone diff for secret exposure, accessibility, destructive reconciliation, and upstream-maintenance impact.
5. Rebuild the retained belief map and commit any audit fixes.
