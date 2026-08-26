# Telegram Authelia

This repository is a maintained fork of [Authelia](https://github.com/authelia/authelia) focused on Forward Auth,
Telegram authentication, passkeys, and simple web administration for small self-hosted installations.

It retains Authelia's reverse-proxy integration, sessions, access-control engine, second factors, and Forward Auth
identity headers. The fork adds a SQL-backed identity model and administration workflows which are not available in
the upstream project.

The source repository is [Montelibero/telegram_authelia](https://github.com/Montelibero/telegram_authelia). The
deployment image is `ghcr.io/montelibero/telegram_authelia:latest`.

> This is an independent fork. It is not an official Authelia release and is not supported by the Authelia project.
> Use the [official Authelia documentation](https://www.authelia.com/) for unchanged upstream functionality.

## Why this fork exists

The target deployment uses Authelia as a Traefik Forward Auth service. Users can authenticate with Telegram, passwords,
or passkeys, while administrators manage identities and application access from the web portal. Email delivery is
optional: administrators can create accounts and copy one-time password setup links directly.

The fork deliberately does not replace Authelia's ACL engine with a separate application authorization system.
Application access remains ordinary Authelia group membership referenced by `access_control.rules`.

## Differences from upstream

### SQL identity backend

- Users, emails, external identities, groups, memberships, password hashes, and audit events are stored in Authelia's
  configured SQL storage.
- SQLite is the primary deployment target. Identity tables live in the same `db.sqlite3` used by Authelia.
- Existing `users_database.yml` accounts can be imported with an idempotent CLI command.
- Usernames are immutable after activation. Primary email selection is controlled by administrators.
- Disabling a user or replacing an external identity invalidates existing sessions while preserving credentials and
  group memberships.

### Telegram authentication

- Telegram Login uses Authorization Code Flow with PKCE against `https://oauth.telegram.org`.
- Local accounts are bound to Telegram's stable numeric profile `id`. The OIDC `sub` claim is still validated but is
  not used as the local identity key.
- An administrator can pre-authorize a user by entering only a Telegram ID and selecting access groups.
- On the first successful Telegram login, the placeholder account is activated automatically. Telegram username and
  display name are recorded without creating a pending approval request.
- When no email was supplied by an administrator, the fork generates `telegram_username@generated_email_domain`.
- Unknown Telegram identities are stored as pending registrations for administrator review.
- Existing users can link, unlink, or replace a Telegram identity. Identity IDs are unique across all users.

### Passkeys and passwords

- Passkeys can be used as an independent one-factor login method when `webauthn.enable_passkey_login` is enabled.
- Password and passkey sessions use the normal Authelia session cookie and Forward Auth flow.
- Authenticated users can manage their display name and password in Security settings.
- Administrators can create an email-only account and immediately copy a one-time password setup link.
- Email notifier support remains available, but the custom setup workflow does not require working outbound email.

### Web administration

Members of the `admins` group receive protected administration pages:

- **Users** creates and edits users, emails, Telegram IDs, status, and group membership.
- **Pending registrations** reviews unknown Telegram identities.
- **Permissions** displays and edits the user/group access matrix.

The groups offered by Users and Permissions are derived from exact `group:*` subjects in `access_control.rules`.
The reserved `admins` group is excluded. The optional legacy `applications` configuration is still accepted for
compatibility, but is not required.

Administrator mutations use optimistic version checks, recent-authentication enforcement, session revocation where
appropriate, and structured audit events. Secrets, password hashes, cookies, OAuth codes, and one-time setup tokens are
not written to audit logs.

### Operational logging

The fork adds structured activity logs for authentication, Telegram resolution and registration, administrator
mutations, redirects, and relevant HTTP activity. Standard Authelia log levels still apply.

## Minimal configuration

The rest of `configuration.yml` is standard Authelia configuration. A minimal custom section looks like this:

```yaml
authentication_backend:
  sql:
    generated_email_domain: eurmtl.me

storage:
  encryption_key: change_me
  local:
    path: /config/db.sqlite3

telegram:
  enabled: true
  issuer: https://oauth.telegram.org
  client_id: change_me
  client_secret: change_me
  callback_url: https://auth.example.com/api/telegram/callback

webauthn:
  enable_passkey_login: true
  display_name: Example

session:
  name: authelia_session
  secret: change_me
  cookies:
    - domain: example.com
      authelia_url: https://auth.example.com
      default_redirection_url: https://auth.example.com/settings/admin/users

access_control:
  default_policy: deny
  rules:
    - domain: grist.example.com
      subject: group:app:grist
      policy: one_factor

    - domain:
        - portainer.example.com
        - dozzle.example.com
      subject: group:app:operations
      policy: one_factor
```

Use actual secret values from the deployment environment. Do not commit secrets to this repository.

## Traefik Forward Auth

Existing Authelia Forward Auth configuration remains compatible:

```yaml
- "traefik.http.middlewares.authelia.forwardauth.address=http://authelia:9091/api/verify?auth=cookie&rd=https://auth.example.com/"
- "traefik.http.middlewares.authelia.forwardauth.trustForwardHeader=true"
- "traefik.http.middlewares.authelia.forwardauth.authResponseHeaders=Remote-User,Remote-Groups,Remote-Name,Remote-Email"
```

Protect an application router with the same middleware:

```yaml
- "traefik.http.routers.example.middlewares=authelia@swarm"
```

Authorization is evaluated by normal Authelia ACL rules. `Remote-Email` is the administrator-selected primary email,
or the generated Telegram email when no explicit email exists.

## Migrating file users to SQL

Preserve `configuration.yml`, `users_database.yml`, and `db.sqlite3` before migration. Stop the running service before
copying SQLite.

Validate the configuration and preview the import:

```sh
/app/authelia validate-config --config /config/configuration.yml
/app/authelia --config /config/configuration.yml storage user import \
  --from /config/users_database.yml \
  --dry-run
```

Run the import only when the preview reports no conflicts:

```sh
/app/authelia --config /config/configuration.yml storage user import \
  --from /config/users_database.yml

/app/authelia --config /config/configuration.yml storage schema-info
/app/authelia --config /config/configuration.yml storage group show admins
```

The import is idempotent. A repeated import should report zero created users and zero conflicts. Keep at least one tested
password-capable member of `admins` for recovery.

Detailed migration and rollback instructions are in
[`local/meta/production-cutover-checklist.md`](local/meta/production-cutover-checklist.md).

## Administrator workflow

1. Open `/settings/admin/users` as a member of `admins`.
2. Create a user with either Telegram ID or email.
3. Select one or more groups derived from ACL rules.
4. For a Telegram-only user, send them to the normal Telegram login. Their first verified login activates the account.
5. For an email-only user, copy the generated one-time setup link and send it through a trusted channel.
6. Use Permissions to review access across all configured application groups.

Users register their own passkeys after they can authenticate. Administrators do not create or transfer passkeys.

## Container image

Every push to `deploy` builds a fresh Linux `amd64` image:

```text
ghcr.io/montelibero/telegram_authelia:latest
```

The image is intentionally published as `latest`; this deployment does not publish release tags. Before updating a
server, retain the currently running image digest so it can be restored if verification fails.

## Branch and update model

The repository keeps upstream history separate from local changes:

```text
upstream/master -> master
stable upstream tag -> release-base
release-base + local/meta + local/auth-overlay + local/ci-deploy -> deploy
```

- `master` is a clean mirror of official Authelia and contains no fork changes.
- `release-base` selects a stable upstream release.
- `local/meta` contains fork documentation and maintenance scripts.
- `local/auth-overlay` contains the custom identity and administration implementation.
- `local/ci-deploy` contains the image build and verification workflow.
- `deploy` is reconstructed from those overlays and is the only branch used to publish the image.

Upstream updates are adopted from stable releases and then the overlays are replayed and tested. The complete procedure
is documented in [`local/meta/fork-overlay-workflow.md`](local/meta/fork-overlay-workflow.md).

## Compatibility and limitations

- The automated image build currently targets Linux `amd64` only.
- SQLite is intended for a single active Authelia instance. A multi-replica deployment requires an appropriate shared
  SQL backend and shared session storage.
- Telegram users must have a public Telegram username when a generated email is required.
- The custom web administration API is for trusted administrators; it is not a public provisioning API.
- Configuration and database backups remain the operator's responsibility.
- Upstream documentation may describe file or LDAP user backends which do not apply when this fork's SQL backend is
  selected.

## Security and support

For vulnerabilities in unchanged Authelia code, follow the
[upstream security policy](https://github.com/authelia/authelia/security/policy). For fork-specific behavior, open a
private security advisory in the Montelibero repository. Do not publish secrets, session cookies, password hashes,
Telegram credentials, OAuth codes, setup links, or private identity headers in issues or logs.

This repository is maintained for Montelibero infrastructure. General upstream support remains available through the
[official Authelia project](https://github.com/authelia/authelia).

## License

The fork remains licensed under the [Apache License 2.0](LICENSE), consistent with upstream Authelia.
