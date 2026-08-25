# Local Deployment Rehearsal

This disposable stack validates the custom image, SQLite user migration, and Caddy Forward Auth without Telegram or production credentials.

The fixtures are development-only:

| Username | Password | Expected result |
| --- | --- | --- |
| `rehearsal-admin` | `RehearsalPass1!` | Explicit email; `admins` and `app:diagnostic` groups |
| `rehearsal-user` | `RehearsalPass2!` | Generated `rehearsal-user@eurmtl.me` email; application access |
| `rehearsal-denied` | `RehearsalPass2!` | Valid login; application ACL denial |
| `rehearsal-disabled` | `DisabledPass1!` | Login denied |

Validate the assets and configurations:

```sh
bash local/deploy/rehearsal/verify.sh
docker compose -f local/deploy/rehearsal/compose.yml config --quiet
docker run --rm -v "$PWD/local/deploy/rehearsal:/config:ro" authelia-mtl:rehearsal /app/authelia validate-config --config /config/configuration-file.yml
docker run --rm -v "$PWD/local/deploy/rehearsal:/config:ro" authelia-mtl:rehearsal /app/authelia validate-config --config /config/configuration-sql.yml
```

Start the file-backend phase:

```sh
docker compose -f local/deploy/rehearsal/compose.yml --profile file up --build --detach
```

Open `https://app.rehearsal.test:8443` after mapping `auth.rehearsal.test` and `app.rehearsal.test` to `127.0.0.1`, or use `curl --resolve` as the automated smoke test does. Caddy redirects an unauthenticated request to `https://auth.rehearsal.test:8443`; after login the private diagnostic endpoint returns only the four identity headers required by the Forward Auth contract. Caddy uses a disposable local CA, so browser trust is local-test-only.

The automated migration, persistence, ACL, and rollback procedure is implemented by `smoke.sh` in the next task. Do not use these fixture secrets outside this disposable stack.
