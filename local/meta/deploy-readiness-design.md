# M6 Deployment Readiness Design

## Goal

M6 turns the completed authentication overlay into a reproducible `linux/amd64` image, rehearses the SQLite and Forward Auth cutover locally without Telegram, and provides an operator-run production procedure that does not require repository or shell access from the implementation agent.

Telegram Bot Login is explicitly deferred to M7. Telegram OIDC is already implemented but is excluded from the local rehearsal because a real provider callback is not available there.

## Branch and image model

Fork-specific CI and deployment test assets live in a dedicated `local/ci-deploy` overlay. The generated `deploy` branch is assembled in this order:

```text
release-base
local/meta
local/auth-overlay
local/ci-deploy
```

Only `deploy` is eligible to publish the deployment image. GitHub Actions builds `linux/amd64` and publishes:

```text
ghcr.io/montelibero/telegram_authelia:latest
```

No release tags are created in M6. The workflow must not modify branches, tags, or repository files.

## Local rehearsal

The disposable local stack contains:

- the custom Authelia image built from the assembled `deploy` tree;
- SQLite on a persistent named volume;
- Caddy as the local Forward Auth reverse proxy;
- a private diagnostic application that returns the identity headers received from Caddy.

The stack uses literal development-only values and is isolated from production. It does not introduce `.env` files or `${VARIABLE}` substitutions. Test configuration and fixtures must contain no production credentials.

The rehearsal proves:

1. file-backed users can be previewed and imported idempotently into SQL;
2. password authentication creates an ordinary Authelia session;
3. Caddy protects the diagnostic application through Forward Auth;
4. `Remote-User`, `Remote-Email`, `Remote-Groups`, and `Remote-Name` are propagated;
5. configured ACL groups grant and deny access correctly;
6. SQLite data survives an Authelia container replacement;
7. the previous file-backed configuration can be restored without deleting the SQLite database or source YAML.

## Production handoff

The production server remains outside agent access. M6 therefore produces an operator-run package containing:

- a Compose service example with literal editable values;
- the required Authelia SQL, session, Telegram OIDC, application, and access-control configuration blocks;
- GHCR pull and restart instructions;
- dry-run, import, idempotency, database integrity, and Forward Auth smoke checks;
- rollback instructions that retain both `users_database.yml` and SQLite;
- a results checklist with explicit stop conditions.

The server configuration must be adapted to its actual reverse proxy separately. The local Caddy stack validates Authelia's Forward Auth contract, not the production proxy syntax.

## Safety and success criteria

- At least one password-capable administrator remains available throughout migration.
- Migration conflicts stop cutover before any backend switch.
- The source YAML, previous image reference, and pre-cutover SQLite backup remain available until two successful deployments have completed.
- Secrets, password hashes, Telegram tokens, cookies, and diagnostic identity headers are not committed or copied into public logs.
- A failed smoke check triggers rollback before additional user or permission changes are made.
- M6 is complete only after the assembled branch passes backend, frontend, image-build, migration, persistence, Forward Auth, and rollback checks.
