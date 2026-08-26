# Telegram Authentication and SQL Users Design

## Goal

Extend the Authelia fork for a private deployment of approximately 10–20 users while preserving the upstream session, Forward Auth, ACL, TOTP, WebAuthn, and authorization behavior.

The fork adds:

- users stored in Authelia's existing SQL database;
- migration from `users_database.yml`;
- Telegram authentication and account linking;
- later pending registration and a small administration interface;
- application permissions represented by ordinary Authelia groups.

The production deployment initially uses one Authelia replica with SQLite. The implementation should continue using Authelia's SQL abstractions where this does not add unnecessary backend-specific work.

## Architecture

```text
Password or Telegram OIDC
        │
        ▼
Immutable local user
        │
        ▼
Standard Authelia session
        │
        ▼
Authelia Forward Auth and ACL
        │
        ▼
Remote-User / Remote-Email / Remote-Groups / Remote-Name
```

The product-specific implementation belongs in `local/auth-overlay`. It must use narrow integration points and avoid modifying Authelia session serialization, Forward Auth response behavior, ACL evaluation, and the OIDC Provider.

Telegram is an upstream identity provider for login. Authelia remains the service that creates the local session and authorizes reverse-proxy requests.

## Delivery milestones

### M0: SQL users

1. Add the overlay-owned SQL schema and migration runner.
2. Implement a SQL-backed `authentication.UserProvider`.
3. Add dry-run and executable import from `users_database.yml`.
4. Verify password login, sessions, Forward Auth headers, and groups against the file backend.
5. Rehearse backup, cutover, and rollback before production migration.

### M1: Telegram login

1. Add Telegram OIDC Authorization Code Flow with PKCE.
2. Allow login only for known, active, linked users.
3. Add password-elevated self-service Telegram linking.
4. Add administrative CLI link and unlink commands.
5. Perform a manual smoke test against the real Telegram service.

### Later milestones

- M2: pending Telegram registration;
- M3: Admin Users UI;
- M4: applications and permissions matrix;
- M5: password management and self-service;
- M6: Telegram Bot Login, only if still required.

## Data model

The overlay uses the same physical database as Authelia but owns a separate migration history in `mtl_schema_migrations`. This avoids migration version collisions with upstream.

### `mtl_users`

- immutable local username;
- display name;
- status: `active`, `disabled`;
- nullable password hash;
- record version and timestamps.

A non-null password hash means password login is enabled. The password hash uses the existing Authelia format and is preserved during migration without rehashing.

### `mtl_user_emails`

- user reference;
- normalized unique email;
- explicit `primary` flag;
- `verified` flag;
- timestamps.

Forward Auth always receives the primary email. During migration, the first existing YAML email becomes primary. If no email exists, the importer creates `<immutable-username>@<generated-email-domain>`.

The generated email domain is infrastructure configuration. The initial deployment uses `eurmtl.me`.

### `mtl_user_identities`

- user reference;
- provider name;
- stable provider user ID;
- current provider username for display;
- timestamps.

The pair of provider and provider user ID is unique. Telegram uses the numeric Telegram user ID as the stable identity. Telegram username, display name, email, or phone number must never be used for automatic account linking.

### Groups

`mtl_groups` and `mtl_group_memberships` store ordinary Authelia group names. Administrative users belong to `admins`. Application permissions use `app:<slug>` groups such as `app:grafana`.

Administrative authority and application access are independent. Membership in `admins` does not grant access to every application.

### Registration and audit

`mtl_registration_requests` stores unknown Telegram identities separately from active users. Approval creates the user and identity transactionally. Rejected requests remain reviewable and do not create repeated pending requests.

`mtl_audit_events` records security-relevant changes without passwords, OAuth codes, tokens, or client secrets.

## User identity and email rules

For a new registration, the current Telegram username may be proposed as the local username. An administrator may change it before approval. After user creation, the local username is immutable even if the Telegram username changes.

`Remote-User` always contains the immutable local username. `Remote-Email` always contains the explicit primary email.

A custom email may initially be changed only by migration or an administrator. Later self-service changes require verification of the new address. Primary email must be unique because downstream applications may use it as their account identifier.

## Telegram authentication

Telegram login uses the official OIDC Authorization Code Flow with:

- exact registered callback URL;
- `state` for request binding and CSRF protection;
- `nonce` for ID token replay protection;
- PKCE using `S256`;
- JWKS signature validation;
- validation of issuer, audience, expiration, and required claims;
- single-use callback state.

The callback is hosted by the Authelia portal, for example `/api/telegram/callback` under the configured public portal URL.

Successful Telegram login resolves a known active local user and creates a normal Authelia session at `one_factor`. Existing `two_factor` ACL policies continue to require the normal TOTP or WebAuthn step.

An unknown Telegram identity does not create a session in M1. The UI explains that the Telegram account is not linked without disclosing another user's local username.

## Account linking

The primary linking flow requires:

1. an existing password-authenticated Authelia session;
2. fresh password elevation;
3. a new Telegram OIDC request bound to the current user;
4. verification that the Telegram identity is not linked elsewhere;
5. transactional creation of the identity and an audit event.

Administrative CLI commands provide link and unlink recovery before the Admin UI exists. They refuse to overwrite an existing identity unless a distinct explicit recovery operation is designed and confirmed.

## Password behavior

Imported users retain their existing password hashes. Telegram-only users have a null password hash.

The preferred password setup and reset mechanism uses the existing one-time reset flow so administrators do not know user passwords. Direct password assignment exists only as an emergency CLI operation.

The system must prevent disabling, deleting, or removing password login from the last active local password administrator.

## Sessions and disabled users

Disabling a user immediately invalidates all existing sessions in addition to blocking new authentication. Re-enabling a user does not restore previously invalidated sessions.

## Administration

The Admin API is hosted under `/api/admin/*`:

- reads require an active user in `admins`;
- mutations require a fresh elevated session and CSRF protection;
- writes use optimistic concurrency and return `409 Conflict` for stale versions;
- user, identity, email, group, password-state, approval, and disable operations create audit events.

The Admin frontend is a lazy-loaded `/admin` route in the existing React portal. Initial screens are Users, Pending, and Permissions.

Applications are initially defined in YAML with name, slug, domain, and group. The UI changes SQL group memberships. Approval alone grants no application access; the default is deny.

## Migration and cutover

The importer has dry-run and executable modes and is idempotent. It preserves usernames, display names, password hashes, emails, and groups. It reports conflicts before mutation.

Production cutover uses a maintenance window:

1. stop Authelia;
2. create and verify a SQLite backup;
3. run the importer in dry-run mode;
4. execute the import;
5. verify users, password hashes, emails, and groups;
6. start the custom binary with the SQL user provider;
7. verify password login and Forward Auth headers.

The original YAML database remains read-only rollback material for at least two successful custom releases. Rollback uses the previous image and configuration. Overlay tables are ignored by the upstream binary.

## Error handling and privacy

User-facing errors expose safe categories such as unavailable login, unlinked account, disabled account, expired authentication, or failed request. Internal details are correlated through a request ID.

Ordinary application logs must not contain passwords, password hashes, OAuth codes, ID or access tokens, client secrets, complete email addresses, or complete Telegram profiles. Full stable provider IDs may appear only in protected audit data when operationally necessary.

Mutations spanning multiple tables are transactional. Unique email or Telegram identity conflicts return controlled domain errors rather than raw database errors.

## Testing

- provider contract tests shared by file and SQL backends;
- SQLite integration tests with real overlay migrations;
- migration tests using realistic `users_database.yml` fixtures;
- password hash preservation and login tests;
- mock OIDC issuer tests for valid and invalid signatures, issuer, audience, expiration, nonce, state, PKCE, replay, and identity collision;
- session and Forward Auth header tests;
- immediate disabled-user session invalidation tests;
- CLI link, unlink, dry-run, import, and recovery tests;
- frontend tests for login, linking, admin, and error states;
- manual Telegram smoke test before the first production rollout.

## Deployment

The GitHub Actions workflow lives in a dedicated `local/ci-deploy` overlay and runs only for `deploy`. Every deploy push builds and publishes:

```text
ghcr.io/montelibero/telegram_authelia:latest
```

Only `linux/amd64` is required. The image records the source commit SHA in OCI labels, and the workflow reports the immutable image digest for rollback. The server is updated manually and GitHub has no production deployment credentials.

Upstream workflows are disabled in the fork through GitHub settings. The image is public when the fork is public and contains no embedded deployment configuration or secrets.
