# Repository Agent Instructions

## Fork workflow

- Read `local/meta/fork-overlay-workflow.md`, `local/meta/BRANCHES.md`, `local/meta/telegram-auth-design.md`, and `local/meta/telegram-auth-implementation-plan.md` before changing authentication code, branches, remotes, or deployment files.
- `master` is a clean mirror of `upstream/master`. Never add local commits to `master`.
- `release-base` points to the selected stable Authelia release and is the base for overlays and deployment.
- Never develop or commit directly on `deploy`. It is a generated integration branch.
- Product-specific authentication work belongs in short-lived topic branches and is integrated into `local/auth-overlay` after review.
- Keep each topic branch focused on one logical change.
- Do not push, force-push, publish images, deploy, or open pull requests without explicit user approval.
- Preserve Authelia's session, Forward Auth, ACL, and authorization behavior unless a task explicitly requires changing them.
- Keep `local/meta/BRANCHES.md` synchronized with active overlay branches and their dependency order.
- A pre-release security fix requires an explicit `security/CVE-*` overlay recorded in `BRANCHES.md`; do not move deployment to arbitrary upstream development commits.

## Architecture discovery

- Use the `codespaces` skill before non-trivial code changes.
- Preserve generated belief-map files and keep them ignored by Git.

## Private local files

- Files under `local/` are private and ignored by default.
- Only the explicitly allowlisted files under `local/meta/` are versioned.
