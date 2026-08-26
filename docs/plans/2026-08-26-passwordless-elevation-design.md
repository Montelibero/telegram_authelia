# Passwordless Elevation Design

## Goal

Allow deployments without working email to disable session-elevation one-time codes and use a recent successful password, passkey, or Telegram authentication for credential-management operations.

## Design

Add `identity_validation.elevated_session.disable_one_time_code`, defaulting to `false` for upstream compatibility. When enabled, a valid one-factor session whose first-factor authentication is no older than `elevation_lifespan` satisfies ordinary elevation checks. Expired sessions must authenticate again; the server must not generate or accept elevation one-time codes while the option is enabled.

Telegram link and unlink use the ordinary elevated-session policy instead of requiring password knowledge specifically. This lets a recent passkey-authenticated session link Telegram and a recent Telegram-authenticated session manage credentials. Existing administrator mutation policy remains unchanged.

The login portal treats a possession/external one-factor session without a protected redirection target as authenticated instead of routing it to the password form. A protected target that actually requires two factors continues through the existing second-factor flow.

## Verification

- Configuration schema and known-key tests cover the new option.
- Middleware tests cover fresh, expired, disabled-code, and default-compatible behavior.
- Handler tests prove one-time-code generation is rejected when disabled.
- Router tests cover passkey/Telegram one-factor navigation with and without a target.
- Existing Go and web test suites must remain green.
