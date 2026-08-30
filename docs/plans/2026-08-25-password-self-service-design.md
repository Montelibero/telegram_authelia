# Password Management and Self-Service Design

## Goal

M5 lets an authenticated local user maintain their password and display name without administrator involvement. It preserves immutable usernames, administrator-controlled email addresses, standard Authelia password policy, Forward Auth behavior, and the existing Telegram identity model.

## Password lifecycle

Users with a password change it by supplying the current password and a policy-compliant replacement. A successful change increments the user's session epoch, invalidating other sessions, while the handler refreshes the current session to the new epoch so it remains active.

Users may remove password login only after supplying the current password and only when a Telegram identity is linked. The last active administrator with a password may not remove it. Removing a password also increments the session epoch and preserves the current session.

Users without a password may create one only after a fresh Telegram OIDC proof. The proof starts from the authenticated session, is bound to the immutable local username and the `password_setup` purpose, uses the existing OIDC state expiry, state cookie, PKCE, nonce, and callback validation, and is consumed once. The returned provider identity must exactly match the Telegram identity already linked to that user.

All passwords are hashed with the configured SQL user-provider algorithm. Existing Authelia password-policy validation remains authoritative.

## Profile self-service

An authenticated active user may change only their display name. Username remains immutable and email management remains administrative because the deployment cannot verify email ownership. Display-name mutation uses same-origin and CSRF protection, optimistic concurrency, and an audit event. The current session is refreshed so Forward Auth receives the new display name through normal user-detail refresh behavior.

## Authorization and safety

- Password change requires the current password.
- First password setup requires a fresh, single-use Telegram proof.
- Password removal requires the current password and a linked Telegram identity.
- The last active password-capable administrator cannot remove password login.
- Display-name mutation requires an authenticated active session but no additional elevation.
- Password hashes, plaintext passwords, OIDC codes, tokens, and proof state are never logged or returned.
- Security-relevant mutations create audit events without secret material.

## User interface

The existing Settings Security page becomes the self-service surface:

- password users see Change Password and Disable Password Login;
- Telegram-only users see Set Password, which starts fresh Telegram verification before displaying password entry;
- every user sees editable Display Name;
- email remains visible through existing user information but is not editable.

The interface reports controlled conflict, stale-version, invalid-proof, and password-policy errors. It does not expose whether another user owns an identity.

## Testing

Tests cover password change, setup and removal, session-epoch rotation with current-session preservation, last password-admin protection, identity mismatch, proof expiry/replay, display-name concurrency, audit events, API authorization and CSRF, frontend states, localization, and a disposable SQLite integration flow.
