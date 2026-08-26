# Deployment Readiness Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Produce and verify the `deploy` branch, publish an amd64 `latest` image through GitHub Actions, rehearse the non-Telegram migration and Forward Auth flow locally, and hand off a safe manual production cutover.

**Architecture:** Deployment automation and rehearsal assets live in `local/ci-deploy`; branch topology and operator documentation live in `local/meta`. The generated `deploy` branch combines the pinned release with both overlays and is the only image publication source. A disposable Caddy stack validates the Forward Auth contract independently of the production proxy.

**Tech Stack:** Git, Bash, GitHub Actions, Docker Buildx, GHCR, Docker Compose, Caddy, SQLite, Go, Vitest.

---

### Task 53: Record the approved M6 design and executable plan

**Files:**
- Create: `local/meta/deploy-readiness-design.md`
- Create: `local/meta/deploy-readiness-implementation-plan.md`

**Steps:**
1. Record the approved branch, image, local-rehearsal, production-handoff, and safety decisions.
2. Review the documents against `local/tz.md`, `local/meta/fork-overlay-workflow.md`, and `local/meta/sql-user-cutover.md`.
3. Run `git diff --check`.
4. Commit the design and plan as clean documentation commits on `local/meta`.

### Task 54: Add the deployment CI overlay to deterministic assembly

**Files:**
- Modify: `local/meta/BRANCHES.md`
- Modify: `local/meta/rebuild-deploy.sh`
- Create: `local/meta/rebuild-deploy-test.sh`

**Steps:**
1. Write a failing shell test that creates a disposable repository, stubs the four required branches, runs the rebuild script, and asserts the merge order and clean-tree refusal.
2. Run the test and confirm it fails because `local/ci-deploy` is absent from the overlay list.
3. Add `local/ci-deploy` after `local/auth-overlay` in both the registry and reconstruction script.
4. Run the shell test and `shellcheck` when available; otherwise run `bash -n` plus the functional test.
5. Commit the meta overlay update.

### Task 55: Add the GitHub Actions image workflow

**Files:**
- Create: `.github/workflows/deploy-image.yml`
- Create: `local/deploy/verify-workflow.sh`

**Steps:**
1. Write a failing workflow-verification script that checks the exact branch trigger, GHCR target, `linux/amd64` platform, `latest` tag, package write permission, and absence of tag/branch mutation steps.
2. Run it against the missing workflow and confirm failure.
3. Add a workflow triggered only by pushes to `deploy` and manual dispatch.
4. Use `docker/login-action`, `docker/setup-buildx-action`, and `docker/build-push-action`; publish `ghcr.io/montelibero/telegram_authelia:latest` for `linux/amd64`.
5. Pin third-party actions to immutable commit SHAs and keep permissions minimal.
6. Run the verifier, YAML parsing, and available workflow security tooling.
7. Commit the CI slice on `local/ci-deploy`.

### Task 56: Verify the assembled production image locally

**Files:**
- Create: `local/deploy/build-image.sh`
- Create: `local/deploy/image-smoke.sh`

**Steps:**
1. Write a failing smoke check for image architecture, `authelia --version`, CLI help, and the SQL user-import command.
2. Add a deterministic local build wrapper using the repository Dockerfile and a local-only image name.
3. Rebuild `deploy` and build the image for `linux/amd64`.
4. Run the image smoke check and record the source commit and image ID without publishing.
5. Commit the scripts.

### Task 57: Build the disposable Caddy and SQLite rehearsal stack

**Files:**
- Create: `local/deploy/rehearsal/compose.yml`
- Create: `local/deploy/rehearsal/configuration-file.yml`
- Create: `local/deploy/rehearsal/configuration-sql.yml`
- Create: `local/deploy/rehearsal/users_database.yml`
- Create: `local/deploy/rehearsal/Caddyfile`
- Create: `local/deploy/rehearsal/diagnostic-app.go`
- Create: `local/deploy/rehearsal/README.md`

**Steps:**
1. Add development-only fixtures containing one administrator, one ordinary user, one disabled user, and explicit/fallback email cases.
2. Use list-form Compose environment entries with literal values; do not add `.env` or `${VARIABLE}` substitutions.
3. Configure Caddy Forward Auth and a private diagnostic application that exposes only the four required identity headers.
4. Validate Compose rendering and both Authelia configurations before starting containers.
5. Start the stack and prove unauthenticated requests redirect to Authelia.
6. Commit the rehearsal stack.

### Task 58: Automate migration, persistence, ACL, and rollback checks

**Files:**
- Create: `local/deploy/rehearsal/smoke.sh`
- Create: `local/deploy/rehearsal/assertions.sh`

**Steps:**
1. Write failing checks for import dry-run counts, first import, idempotent second import, password login, disabled-user denial, Forward Auth headers, ACL denial, and SQLite persistence.
2. Add an explicit rollback phase that returns to the file backend without removing SQLite or YAML.
3. Run the complete rehearsal twice from clean disposable volumes.
4. Verify cleanup affects only resources labeled for the rehearsal project.
5. Commit the automated smoke suite.

### Task 59: Prepare the manual production cutover package

**Files:**
- Modify: `local/meta/sql-user-cutover.md`
- Create: `local/meta/server-compose-example.yml`
- Create: `local/meta/production-cutover-checklist.md`

**Steps:**
1. Add the exact GHCR image, pull, backup, dry-run, import, restart, Forward Auth, and rollback sequence.
2. Use literal editable Compose values, list-form `environment`, no `.env`, and no variable substitutions.
3. Separate required Authelia configuration from reverse-proxy-specific syntax.
4. Add stop conditions for conflicts, integrity failures, missing emergency admin, missing forwarded email, and failed rollback smoke.
5. Validate Compose syntax and all copy-paste commands in a disposable directory.
6. Commit the operator handoff.

### Task 60: Audit, integrate, and hand off external actions

**Files:**
- Modify: `local/meta/BRANCHES.md`

**Steps:**
1. Run affected Go race tests, all web tests, ESLint, TypeScript, image build, workflow verification, and the complete rehearsal.
2. Request an independent review focused on workflow permissions, secret exposure, branch safety, migration atomicity, persistent storage, and rollback.
3. Fix every Critical or Important finding and rerun the affected checks.
4. Integrate `local/ci-deploy`, rebuild `deploy`, and prove overlay ancestry/order.
5. Mark M6 locally ready in the branch registry.
6. Stop before pushing. Request explicit approval for publishing `local/meta`, `local/auth-overlay`, `local/ci-deploy`, and the generated `deploy` branch.
7. After the user performs the separate server cutover, record only non-sensitive results and remaining issues.
