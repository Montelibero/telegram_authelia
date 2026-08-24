# Authelia Fork Overlay Workflow

This repository uses an overlay workflow that keeps the official Authelia history easy to update while isolating Montelibero-specific authentication and deployment changes.

## Repository model

```text
upstream/master ───────────── official Authelia history
       │
       ▼ fast-forward only
master ────────────────────── clean mirror of upstream/master
       │ select stable tag
       ▼
release-base ──────────────── selected stable Authelia release
       ├── local/meta
       ├── local/auth-overlay
       ├── optional security/CVE-* hotfix
       └── other focused local/* overlays
                    │
                    ▼ deterministic assembly
deploy ────────────────────── exact source used for deployment
```

The remotes are:

```text
fork      https://github.com/Montelibero/authelia.git
upstream  https://github.com/authelia/authelia.git
```

Push access to `upstream` must remain disabled.

## Branch roles

### `master`

- Exactly mirrors `upstream/master`.
- Receives only fast-forward updates from upstream.
- Never contains local commits, documentation, CI changes, or deployment configuration.
- Is not a deployment branch.

### `release-base`

- Points to the selected stable Authelia release tag.
- Is the only normal base for topic branches, overlays, and `deploy`.
- Contains no local commits.
- Moves only when a new stable release is deliberately selected.
- Must not be force-pushed or used for development.

### Topic branches

Short-lived branches are created from the current `release-base` for one logical change:

```text
feat/sql-user-provider
feat/yaml-user-migration
feat/telegram-login
fix/telegram-session
```

Topic branches should have reviewable commits and tests. A change suitable for official Authelia may be proposed upstream. Private product behavior is integrated into `local/auth-overlay`.

### `local/auth-overlay`

This is the long-lived product overlay for the custom authentication system. It may contain the integrated results of dependent topic branches, including:

- SQL-backed users, identities, groups, and memberships;
- Telegram login and account linking;
- migration from `users_database.yml`;
- custom login UI integration;
- later registration and administration features.

The overlay must avoid changing Authelia session serialization, Forward Auth, ACL evaluation, and authorization behavior unless no stable integration boundary exists.

### `security/CVE-*`

A temporary security overlay is allowed when upstream has published a confirmed security fix but has not yet released a stable tag containing it.

- Cherry-pick only the required upstream fix and its necessary tests.
- Record the source commit, advisory, and removal condition in `BRANCHES.md`.
- Verify it independently and again as part of the assembled `deploy` branch.
- Remove the overlay after moving `release-base` to a stable release containing the fix.
- Never replace this process by deploying an arbitrary upstream development snapshot.

### `local/meta`

This branch contains only fork-maintenance material:

- `AGENTS.md`;
- `local/meta/fork-overlay-workflow.md`;
- `local/meta/BRANCHES.md`;
- `local/meta/rebuild-deploy.sh`.

It must not contain product code.

### `deploy`

- Is generated from `release-base` plus the active overlays listed in `BRANCHES.md`.
- Is the only branch used by the deployment build.
- Must never receive hand-written commits.
- May be force-pushed after deterministic reconstruction.
- Must be tagged after successful verification, for example `mtl-v4.39.20-1`.

## Hard rules

1. Never commit directly to `master` or `deploy`.
2. Never push or publish without explicit user approval.
3. Update `release-base` from stable upstream release tags; do not deploy arbitrary upstream development commits.
4. Keep topic branches small and focused.
5. Record every active overlay and dependency in `BRANCHES.md`.
6. Rebuild `deploy`; do not repair it manually.
7. Run the relevant backend and frontend tests before promoting a build.
8. Preserve a known password-based emergency administrator during authentication migrations.
9. Keep `rerere` enabled to reuse conflict resolutions during repeated upstream updates.

## Stable upstream update cycle

Choose a stable Authelia release tag before starting. Security fixes may be applied separately through a recorded `security/CVE-*` overlay when waiting for the next stable release is not acceptable.

```bash
git fetch upstream --tags
git switch master
git merge --ff-only upstream/master
git switch release-base
git merge --ff-only <stable-upstream-tag>
```

Verify that `master` contains no local commits:

```bash
git log upstream/master..master --oneline
```

Rebase each active overlay onto the updated `release-base`, starting with the roots listed in `BRANCHES.md`. Resolve and test each overlay before moving to its dependants.

After all overlays pass their checks, reconstruct `deploy` with `local/meta/rebuild-deploy.sh`. Inspect the resulting log and diff before any push.

## Deploy reconstruction

The reconstruction script is intentionally local-only:

- it checks prerequisites;
- it rebuilds `deploy` from `release-base` and the configured overlays;
- it never fetches, pushes, tags, publishes, or deploys;
- it refuses to run with a dirty worktree.

Because reconstruction resets the local `deploy` branch, review the script and ensure any valuable deploy-only commits have first been moved to an overlay branch.

## Promotion and rollback

After the assembled `deploy` branch passes tests:

1. inspect the full diff from the selected upstream tag;
2. create an immutable custom tag such as `mtl-v4.39.20-1`;
3. push `deploy` and the tag only after explicit approval;
4. build an image tagged with the same custom version;
5. retain the previous image or digest for rollback.

Do not rely on `latest` as the only deployable image reference.

## CI policy

- Upstream CI must not run merely because `master` mirrors upstream.
- Fork-specific CI belongs in a focused overlay such as `local/ci-deploy`.
- Deployment workflows must trigger only for `deploy` or approved custom tags.
- A CI workflow must not silently mutate `master`, overlay branches, or tags.

## Adding a new overlay

1. Create a focused topic branch from the current `release-base`.
2. Implement and verify the change.
3. Decide whether it belongs upstream or in a long-lived local overlay.
4. Integrate private authentication work into `local/auth-overlay`.
5. Add or update its entry in `BRANCHES.md`.
6. Rebuild and verify `deploy`.

## Removing an overlay

Remove an overlay when the change is accepted upstream, becomes unnecessary, or can no longer be maintained safely. Delete it from the deploy order, reconstruct `deploy`, and verify the resulting behavior before deleting the branch.

## Operational checklist

- `master` has no commits ahead of `upstream/master`.
- `release-base` resolves to the selected stable upstream release.
- `upstream` has no push URL.
- Every active overlay appears in `BRANCHES.md`.
- Overlay dependency order is explicit.
- `deploy` was reconstructed rather than edited.
- Relevant tests pass on the assembled tree.
- The release has an immutable custom tag and image reference.
- Push and deployment have explicit approval.

## Branch protection policy

Configure protection after the branches are first published:

- `master`: forbid force-push and deletion;
- `release-base`: forbid force-push, deletion, and direct development commits;
- `deploy`: allow force-push only for the repository owner or designated automation actor;
- topic and local overlay branches: leave unprotected unless a later workflow requires reviews.
