# Production Cutover Checklist

This checklist deploys `ghcr.io/montelibero/authelia:latest` on `linux/amd64`, migrates the existing file-backed users into the SQLite-backed user provider, and verifies the Forward Auth identity contract. The server operator runs every command; the implementation agent has no server access.

The Compose example is deliberately proxy-neutral. Replace its external volume and network names with the existing server resources in the deployment UI. Keep the current reverse-proxy labels or routing configuration unchanged except for the Forward Auth endpoint and copied identity headers.

## STOP conditions

Stop before switching the authentication backend when any of the following is true:

- the current image reference, configuration, `users_database.yml`, or stopped SQLite copy has not been preserved;
- the image architecture is not `amd64`;
- configuration validation fails;
- import preview reports any conflict;
- no enabled password-capable user belongs to `admins` after import;
- a second import reports anything other than zero created and zero conflicts;
- SQLite cannot be opened by `storage schema-info`;
- the previous image and file-backend configuration cannot be restored;
- Forward Auth does not return the expected `Remote-Email` for both an explicit-email user and a fallback-email user.

## Required configuration

Keep the existing server, storage, notifier, and reverse-proxy settings unless this checklist explicitly changes them. The M6 authentication block is:

```yaml
authentication_backend:
  password_reset:
    disable: true
  sql:
    generated_email_domain: eurmtl.me
```

Keep SQLite on the persistent config volume:

```yaml
storage:
  local:
    path: /config/db.sqlite3
```

Keep the portal and applications inside the same secure cookie domain:

```yaml
session:
  cookies:
    - domain: eurmtl.me
      authelia_url: https://auth.eurmtl.me
      default_redirection_url: https://change_me.eurmtl.me
      same_site: lax
```

Applications shown in the admin permissions matrix and their ACL rules use the same group:

```yaml
applications:
  - slug: change_me
    name: Change Me
    domain: change_me.eurmtl.me
    group: app:change_me
    enabled: true

access_control:
  default_policy: deny
  rules:
    - domain: change_me.eurmtl.me
      subject: group:app:change_me
      policy: one_factor
```

Telegram remains disabled for M6. M7 will enable and verify it separately:

```yaml
telegram:
  enabled: false
  issuer: https://oauth.telegram.org
  client_id: change_me
  callback_url: https://auth.eurmtl.me/api/telegram/callback
```

Secrets remain in the deployment UI environment settings, not in `configuration.yml`. Replace every `change_me` value in the Compose example before starting it.

## 1. Record the current state

Record privately, without copying secrets or password hashes into tickets or chat:

- current image reference and image ID;
- config-volume name and reverse-proxy network name;
- current health result and one successful password login;
- checksums of `/config/configuration.yml`, `/config/users_database.yml`, and `/config/db.sqlite3` when present.

Confirm the source file contains at least one enabled password-capable administrator in the `admins` group.

## 2. Pull and inspect without switching

```sh
docker compose pull authelia
docker image inspect ghcr.io/montelibero/authelia:latest --format '{{.Architecture}} {{.Id}}'
docker compose run --rm authelia /app/authelia --version
docker compose run --rm authelia /app/authelia storage user import --help
```

The architecture must be `amd64`, and the custom `storage user import` command must exist.

## 3. Stop and back up

Stop Authelia before copying SQLite so the copy represents one consistent point in time:

```sh
docker compose stop authelia
docker compose run --rm --entrypoint sh authelia -c 'mkdir -p /config/backups && cp /config/db.sqlite3 /config/backups/authelia-before-sql-users.sqlite3 && sha256sum /config/configuration.yml /config/users_database.yml /config/db.sqlite3 /config/backups/authelia-before-sql-users.sqlite3'
```

If `/config/db.sqlite3` does not yet exist, omit it from the copy command, preserve the other two files, and let the custom image create SQLite during the preview. Do not remove or edit `users_database.yml`.

## 4. Validate and preview

Keep a temporary copy of the current configuration with the file backend for rollback. Validate the SQL version and preview the import:

```sh
docker compose run --rm authelia /app/authelia validate-config --config /config/configuration.yml
docker compose run --rm authelia /app/authelia --config /config/configuration.yml storage user import --from /config/users_database.yml --dry-run
```

STOP on any conflict. The report must not contain password hashes or email addresses.

## 5. Import and prove idempotency

```sh
docker compose run --rm authelia /app/authelia --config /config/configuration.yml storage user import --from /config/users_database.yml
docker compose run --rm authelia /app/authelia --config /config/configuration.yml storage user import --from /config/users_database.yml
docker compose run --rm authelia /app/authelia --config /config/configuration.yml storage schema-info
docker compose run --rm authelia /app/authelia --config /config/configuration.yml storage group show admins
```

The second import must report zero created and zero conflicts. The `admins` group must contain the expected emergency administrators.

## 6. Start and test Forward Auth

```sh
docker compose up --detach authelia
docker compose ps authelia
docker compose logs --tail 100 authelia
```

Wait for the health check. Then perform these private browser checks:

1. Sign in with an existing ordinary password account.
2. Confirm a disabled account is rejected.
3. Open an allowed application and inspect its private diagnostic output or logs.
4. Confirm exactly `Remote-User`, `Remote-Email`, `Remote-Groups`, and `Remote-Name` arrive from Forward Auth.
5. Confirm an explicit source email remains primary.
6. Confirm a user without an explicit source email receives `username@eurmtl.me`.
7. Confirm a signed-in user outside the application's group is denied.
8. Sign in as an administrator and confirm the Users, Pending, Groups, and Permissions pages load.

Do not expose diagnostic identity headers on a public route.

## Rollback

Rollback immediately when startup, login, admin access, email propagation, ACL, or persistence checks fail:

1. Stop the custom image.
2. Restore the recorded previous image reference in the deployment UI.
3. Restore the saved file-backend `authentication_backend.file` configuration.
4. Keep both `users_database.yml` and the populated SQLite database.
5. Start the previous image and repeat password login plus Forward Auth checks.

If SQLite itself cannot be opened, restore the stopped copy only while Authelia is stopped:

```sh
docker compose stop authelia
docker compose run --rm --entrypoint sh authelia -c 'cp /config/backups/authelia-before-sql-users.sqlite3 /config/db.sqlite3 && sha256sum /config/db.sqlite3'
docker compose up --detach authelia
```

Do not delete the failed SQLite database or YAML source. Preserve them for diagnosis. Keep the previous image reference and backups through two successful deployments.

## Result record

Record privately:

- image ID and deployment time;
- import created/unchanged/conflict counts;
- successful ordinary/admin login results;
- explicit and fallback email propagation results;
- allowed and denied ACL results;
- container replacement persistence result;
- rollback smoke result;
- any remaining issue, without secrets, cookies, hashes, Telegram tokens, or identity-header values.
