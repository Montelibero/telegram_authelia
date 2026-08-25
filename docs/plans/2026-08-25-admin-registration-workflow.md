# Admin Registration Workflow Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development to implement this plan task-by-task.

**Goal:** Let administrators inspect, approve, and reject pending external registrations from the protected admin API.

**Architecture:** Extend the existing registration storage transaction so approval accepts administrator-edited profile fields and an explicit group list. Expose thin JSON handlers behind the existing admin read and mutation middleware, preserving optimistic version checks and the CLI contract.

**Tech Stack:** Go, fasthttp, SQLx, SQLite, testify.

---

### Task 1: Extend registration approval storage

**Files:**
- Modify: `internal/model/mtl_registration.go`
- Modify: `internal/storage/mtl_registrations.go`
- Test: `internal/storage/mtl_registrations_test.go`

1. Write failing tests for edited display name, explicit groups, absent default groups, stale versions, and rollback when a requested group does not exist.
2. Run the focused storage tests and confirm the new assertions fail.
3. Add `DisplayName` and `Groups` to the approval model and insert memberships inside the existing transaction.
4. Run storage tests and race tests until green.

### Task 2: Add protected admin registration endpoints

**Files:**
- Create: `internal/handlers/handler_admin_registrations.go`
- Create: `internal/handlers/handler_admin_registrations_test.go`
- Modify: `internal/server/handlers.go`
- Modify: `internal/server/handlers_test.go`

1. Write failing lifecycle tests for status-filtered list/detail, approval, rejection, stale mutations, and safe response DTOs.
2. Add read routes using `RequireAdmin` and mutation routes using `RequireAdminMutation`.
3. Keep request identifiers in query strings or JSON bodies so provider values remain unrestricted.
4. Run handler, server route, storage, and race tests.

### Task 3: Verify and commit

1. Rebuild `.belief_map.sexp` and retain its ignored cache.
2. Run `git diff --check`, focused race tests, and the server route subset.
3. Inspect the staged diff for secrets and unsafe response fields.
4. Commit the complete Task 32 slice.
