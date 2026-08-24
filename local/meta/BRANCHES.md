# Active Overlay Branches

This file records the branches used to assemble `deploy`. Update it whenever an overlay is added, removed, renamed, or changes dependency order.

## Upstream baseline

| Item | Value |
|---|---|
| Mirror branch | `master` |
| Upstream branch | `upstream/master` |
| Stable baseline branch | `release-base` |
| Update policy | Stable release tags only |
| Current selected release | Not selected |

## Active overlays

| Order | Branch | Type | Depends on | Purpose | Status |
|---:|---|---|---|---|---|
| 10 | `local/meta` | local | `release-base` | Fork workflow, branch registry, and agent instructions | Planned |
| 20 | `local/auth-overlay` | local | `release-base` | SQL users, Telegram authentication, migration, and later admin features | Planned |

Temporary `security/CVE-*` overlays are inserted after `local/meta` and before product overlays. Each entry must identify the upstream commit and stable release that permits removal.

## Planned topic branches

| Branch | Integration target | Purpose | Status |
|---|---|---|---|
| `feat/sql-user-provider` | `local/auth-overlay` | Store users, identities, groups, and memberships in Authelia SQL storage | Planned |
| `feat/yaml-user-migration` | `local/auth-overlay` | Dry-run and import users from `users_database.yml` | Planned |
| `feat/telegram-account-linking` | `local/auth-overlay` | Link a stable Telegram numeric ID to an existing local user | Planned |
| `feat/telegram-login` | `local/auth-overlay` | Telegram OIDC login with Authorization Code Flow and PKCE | Planned |

## Deploy order

```text
release-base
local/meta
local/auth-overlay
```

The executable order in `rebuild-deploy.sh` must match this list.

## Release history

| Custom tag | Upstream baseline | Image | Notes |
|---|---|---|---|
| — | — | — | No custom release yet |
