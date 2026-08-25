# Telegram Pending Registration Design

## Scope

M2 records unknown Telegram OIDC identities as registration requests without creating an Authelia user or session. It adds CLI review and approval before the Admin UI planned for M3.

## Registration flow

After successful Telegram OIDC verification, login first resolves the stable numeric Telegram user ID against `mtl_user_identities`. A known active user follows the existing M1 login path. An unknown identity is upserted into `mtl_registration_requests` and redirected back to the portal with a safe `pending` result.

The stable provider user ID identifies a request. Repeated login updates current display data without creating duplicates. A rejected request remains rejected and reviewable; repeated login does not silently reopen it. Pending and rejected identities never create a user, session, Forward Auth identity, email, or group membership.

## Proposed identity and email

The current Telegram username is the default immutable local username proposal. A verified email returned by Telegram may be retained as the proposed primary email. Otherwise, when a Telegram username exists, the proposed email is `<telegram-username>@<generated-email-domain>`.

A request without a Telegram username is still stored. Approval then requires explicit `--username` and `--email` values. Approval may also override automatically proposed values. Username and primary email uniqueness are checked transactionally.

## State and approval

Requests have `pending`, `approved`, or `rejected` status, optimistic versioning, request/update timestamps, and resolution metadata. Approval atomically creates:

- one active passwordless `mtl_users` row;
- one verified primary email;
- one Telegram identity;
- no groups;
- approval and user-creation audit events;
- the terminal approved request state.

Approval never grants `admins` or `app:*`. The next Telegram login creates the ordinary M1 one-factor session. Reject changes only the request and audit history.

## CLI

The command group is `authelia storage user registration` with `list`, `show`, `approve`, and `reject`. Every level has Cobra help, examples, and discoverable subcommands. Running the group without a subcommand prints help instead of returning an opaque error.

Approval accepts the exact request ID and expected version. Optional `--username` and `--email` override proposals. Reject accepts the exact request ID and expected version. Output excludes OAuth codes, tokens, secrets, and complete Telegram profiles.

## Error handling and tests

Controlled domain errors cover missing requests, stale versions, terminal requests, username/email/identity conflicts, and incomplete approval data. Tests cover migrations, idempotent upsert, rejected replay, no-username requests, atomic approval, rollback, CLI help and commands, login/session behavior, Forward Auth after approval, and privacy-safe output.
