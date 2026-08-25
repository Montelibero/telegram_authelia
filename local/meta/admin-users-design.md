# Admin Users Design

## Goal

M3 adds the daily administration surface for the private Authelia deployment. Administrators use the existing portal to create and maintain users, approve Telegram registrations, manage identities, email addresses, groups, and access memberships. CLI commands exist only for initial bootstrap and emergency recovery.

## Authorization and elevation

The Admin API is hosted under `/api/admin/*`; the frontend is a lazy-loaded `/admin` route in the existing React application. Both the navigation link and API reads require an active local user in the `admins` group. The API always performs its own authorization check.

Reads are available from any authenticated admin session, including a Telegram-created one-factor session. Mutations require the existing fresh password elevation flow inside the current session. Elevation does not log the administrator out or restart Telegram OIDC. An admin without a password hash has read-only access until a password is configured.

All mutations use the existing same-origin and CSRF protections, optimistic concurrency, SQL transactions, and audit events. A stale expected version returns `409 Conflict`. The system intentionally permits disabling the last administrator or removing the last administrator membership; CLI recovery is the accepted escape hatch.

## Users

The Users screen loads the complete user set because the initial deployment has approximately 10–20 users. Local search covers immutable username, display name, and email. There is no product cap or artificial pagination limit.

Administrators can:

- create a user with immutable username, display name, primary email, and groups;
- edit display name and status;
- add and remove verified email addresses and select one unique primary email;
- inspect groups and Telegram identities;
- add and remove group memberships;
- unlink Telegram after an explicit warning when it is the last login method;
- disable or enable a user;
- create a short-lived, single-use password setup link.

Hard deletion is not part of M3. Disabled records and their audit trail remain. Display name and email changes rely on Authelia's normal user-detail refresh; they do not force logout. Disable immediately prevents authentication and invalidates existing sessions.

Dangerous actions against the current administrator—disabling self, removing the `admins` membership, or unlinking the final login method—require typing the current username. This is confirmation, not a business prohibition.

## Registrations

The Pending screen has Pending, Approved, and Rejected tabs. Approval allows the administrator to edit the proposed immutable username, display name, primary email, and initial groups. No groups are selected automatically. Approval atomically creates the user, verified primary email, provider identity, memberships, resolution metadata, and audit events.

Reject preserves the request and its provider metadata for review. Concurrent or stale approval/rejection returns `409 Conflict` and the UI reloads the request instead of overwriting another administrator's decision.

## Groups and access

M3 provides complete group administration in the web UI and recovery CLI:

- list, inspect, create, rename, and delete groups;
- add and remove user memberships;
- show affected users before rename or delete.

Group names are unrestricted except for technical storage/protocol requirements. `admins` and `app:*` are conventions, not validation rules. SQL cannot know whether a group is referenced by YAML ACL configuration; rename and delete therefore show an explicit warning that external configuration is not updated automatically.

M4 remains a higher-level application catalog and a visual users-by-applications matrix. M3 already provides all underlying access control operations.

## Password setup

The existing reset/setup completion path remains the only normal way to choose a password. M3 adds an admin-authorized operation that creates a short-lived, single-use setup link and displays it once for copying through another channel. The deployment has no working email and M3 does not add mail infrastructure. Existing notifier behavior is not intentionally disabled.

The link contains an opaque one-time token, never the password. Token creation, use, expiry, and replay are audited without logging the token.

## Session invalidation

Session backends do not expose a portable username-wide deletion operation. M3 therefore adds an overlay-owned per-user session epoch/version to `mtl_users`. The authenticated session records the epoch observed at login; request-time user refresh rejects and destroys a session when the stored epoch differs. Disabling a user increments the epoch transactionally. This provides backend-independent immediate logical revocation without scanning Redis or cookie storage.

## API shape

The initial resources are:

- `GET/POST /api/admin/users`;
- `GET/PATCH /api/admin/users/{username}`;
- email, identity, group membership, status, and password-setup subresources;
- `GET/POST/PATCH/DELETE /api/admin/groups` and group resources;
- `GET /api/admin/registrations`;
- approval and rejection mutation resources.

Responses use dedicated admin DTOs and never expose password hashes, OAuth codes, tokens, secrets, or complete provider profiles. Password setup is the sole response containing a one-time setup URL.

## Testing

Tests cover SQL transactions and conflicts, admin/read/elevation middleware, CSRF behavior, session epoch revocation, self-action confirmation, group warnings and memberships, registration approval with groups, password setup expiry/replay, prefix-aware routes, and React screens in light/dark/system themes. A disposable SQLite integration flow proves bootstrap admin, web-created user, pending approval, access assignment, disable, and recovery CLI.

