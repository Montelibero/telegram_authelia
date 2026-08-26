# SQL User Cutover Runbook

This runbook migrates an Authelia file user database to the overlay-owned SQL user backend. M0 is rehearsed with SQLite. Keep `users_database.yml` unchanged and readable by the previous upstream image until at least two successful custom-image deployments have completed.

Replace the example paths and container names with the deployment's actual values. Run all commands inside the Authelia container or an equivalent one-off container with the same `/config` volume mounted. Do not place storage encryption keys or password hashes in terminal transcripts or repository files.

## Preconditions

- The custom image is `ghcr.io/montelibero/telegram_authelia:latest`, built only from the assembled `deploy` branch on the pinned stable baseline.
- The current upstream deployment is healthy with the file authentication backend.
- `/config/users_database.yml` is the exact file currently used by Authelia.
- SQLite storage is configured at `/config/db.sqlite3`.
- The custom configuration contains:

```yaml
authentication_backend:
  sql:
    generated_email_domain: eurmtl.me
```

The generated domain is only a fallback. An explicit email from `users_database.yml` remains the user's primary verified email; otherwise username `bublik` receives `bublik@eurmtl.me` as an unverified fallback address.

## 1. Back up and verify SQLite

Stop Authelia so the backup and source files describe one cutover point, then make a byte-for-byte SQLite copy:

```sh
mkdir -p /config/backups
cp /config/db.sqlite3 /config/backups/authelia-before-sql-users.sqlite3
sha256sum /config/db.sqlite3 /config/backups/authelia-before-sql-users.sqlite3 /config/users_database.yml
```

The source and copied SQLite checksums must match. The runtime image does not include the `sqlite3` utility; use `authelia storage schema-info` after opening the copied database rather than assuming it exists in the container. Store the observed checksums and user counts in a private operations log; do not commit them.

## 2. Run the import preview

Run the custom binary against the existing configuration and YAML database:

```sh
authelia --config /config/configuration.yml storage user import --from /config/users_database.yml --dry-run
```

Review every `Created`, `Unchanged`, and `Conflict` entry. A conflict must be resolved before cutover; the executable import performs no partial write when conflicts exist. The report never prints password hashes or email addresses.

## 3. Import and verify idempotency

Execute the import once:

```sh
authelia --config /config/configuration.yml storage user import --from /config/users_database.yml
```

Run the same command a second time. Every imported username must be reported as `Unchanged`, with zero `Created` and zero `Conflicts`.

Record private verification counts without selecting password hashes:

```sh
sqlite3 /config/db.sqlite3 'SELECT status, COUNT(*) FROM mtl_users GROUP BY status ORDER BY status;'
sqlite3 /config/db.sqlite3 'SELECT COUNT(*) FROM mtl_user_emails;'
sqlite3 /config/db.sqlite3 'SELECT COUNT(*) FROM mtl_groups;'
sqlite3 /config/db.sqlite3 'SELECT COUNT(*) FROM mtl_group_memberships;'
```

The SQL queries are optional diagnostics and require a separately available `sqlite3` utility. They are not runtime-image commands. The required image-native check is:

```sh
authelia --config /config/configuration.yml storage schema-info
```

## 4. Start the SQL backend

Keep the SQLite path and encryption key unchanged. Replace the file backend configuration with the SQL backend block shown in the preconditions, deploy the custom image, and start Authelia.

Confirm that startup applies the independent `mtl_schema_migrations` series without modifying Authelia's upstream schema history.

## 5. Smoke test

Use a non-administrator test account first, then an administrator account if one exists:

1. Sign in with the existing password.
2. Confirm a disabled account is rejected.
3. Open an application protected by Authelia Forward Auth.
4. Confirm the application receives the expected `Remote-User`, `Remote-Email`, `Remote-Groups`, and `Remote-Name` values.
5. Confirm the primary explicit email is forwarded when present; otherwise confirm the generated fallback address.
6. Confirm existing second-factor enrollment and protected applications still work.

Do not inspect these headers through a public endpoint. Use application logs or a private diagnostic endpoint and remove any temporary diagnostic route after verification.

## 6. Roll back

If startup or a smoke test fails:

1. Stop the custom image.
2. Restore the previous upstream image and its file authentication backend configuration.
3. Keep `/config/users_database.yml` in place and unchanged.
4. Start the upstream image and repeat password and Forward Auth smoke tests.

The upstream image ignores the overlay-owned `mtl_*` tables, so restoring the database is not normally required. If the SQLite file itself is damaged, stop Authelia, preserve the failed database, and restore the verified backup:

```sh
test -s /config/backups/authelia-before-sql-users.sqlite3
failed=/config/backups/authelia-failed-$(date -u +%Y%m%dT%H%M%SZ).sqlite3
cp /config/db.sqlite3 "$failed"
cp /config/backups/authelia-before-sql-users.sqlite3 /config/db.sqlite3
sha256sum "$failed" /config/backups/authelia-before-sql-users.sqlite3 /config/db.sqlite3
authelia --config /config/configuration.yml storage schema-info
```

Compare the restored checksum with the checksum recorded when the backup was created. If the backup is missing or empty, do not overwrite the failed database; keep Authelia stopped and recover storage separately. Do not delete the quarantined failed database or YAML source during rollback. Preserve both for diagnosis.

## Rehearsal record

Before each production cutover, rehearse dry-run, import, repeat import, SQL startup, Forward Auth, and rollback against disposable copies of the production-shaped SQLite database and YAML file. Keep actual row counts, usernames, checksums, and incident notes in the private operations log only.
