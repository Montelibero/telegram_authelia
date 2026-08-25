import { useCallback, useEffect, useState } from "react";

import {
    Alert,
    Box,
    Button,
    Card,
    CardContent,
    Chip,
    Dialog,
    DialogActions,
    DialogContent,
    DialogTitle,
    Divider,
    FormControlLabel,
    List,
    ListItem,
    ListItemButton,
    ListItemText,
    MenuItem,
    Stack,
    Switch,
    TextField,
    Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { useNotifications } from "@contexts/NotificationsContext";
import {
    AdminUserCreate,
    AdminUserDetails,
    AdminUserSummary,
    addAdminGroupUser,
    addAdminUserEmail,
    createAdminUser,
    deleteAdminUserEmail,
    generateAdminUserSetupLink,
    getAdminGroup,
    getAdminUser,
    getAdminUsers,
    removeAdminGroupUser,
    setAdminUserPrimaryEmail,
    unlinkAdminUserIdentity,
    updateAdminUser,
} from "@services/Admin";
import { postFirstFactorReauthenticate } from "@services/Password";

interface UsersViewProps {
    currentUsername: string;
}

const UsersView = function ({ currentUsername }: UsersViewProps) {
    const { t: translate } = useTranslation("settings");
    const { createErrorNotification, createSuccessNotification } = useNotifications();
    const [users, setUsers] = useState<AdminUserSummary[]>([]);
    const [selected, setSelected] = useState<AdminUserDetails>();
    const [loading, setLoading] = useState(true);
    const [createOpen, setCreateOpen] = useState(false);
    const [password, setPassword] = useState("");
    const [filter, setFilter] = useState("");

    const loadUsers = useCallback(async () => {
        try {
            setUsers(await getAdminUsers());
        } catch {
            createErrorNotification(translate("Failed to load users"));
        } finally {
            setLoading(false);
        }
    }, [createErrorNotification, translate]);

    useEffect(() => {
        loadUsers().catch(console.error);
    }, [loadUsers]);

    const openUser = useCallback(
        async (username: string) => {
            try {
                const details = await getAdminUser(username);
                setSelected({
                    ...details,
                    emails: details.emails ?? [],
                    groups: details.groups ?? [],
                    identities: details.identities ?? [],
                });
            } catch {
                createErrorNotification(translate("Failed to load user"));
            }
        },
        [createErrorNotification, translate],
    );

    const unlock = useCallback(async () => {
        try {
            await postFirstFactorReauthenticate(password);
            setPassword("");
            createSuccessNotification(translate("Administrator actions unlocked"));
        } catch {
            createErrorNotification(translate("Incorrect password or reauthentication failed"));
        }
    }, [createErrorNotification, createSuccessNotification, password, translate]);

    const applyDetails = useCallback(
        async (operation: () => Promise<AdminUserDetails>) => {
            try {
                const details = await operation();
                setSelected(details);
                await loadUsers();
                createSuccessNotification(translate("User updated"));
            } catch (error) {
                if ((error as { response?: { status?: number } }).response?.status === 409 && selected) {
                    try {
                        setSelected(await getAdminUser(selected.username));
                        await loadUsers();
                        createErrorNotification(
                            translate("User changed elsewhere; the latest version has been loaded"),
                        );
                        return;
                    } catch {
                        // Fall through to the generic error when the conflict refresh also fails.
                    }
                }
                createErrorNotification(translate("User update failed; reauthenticate or reload and try again"));
            }
        },
        [createErrorNotification, createSuccessNotification, loadUsers, selected, translate],
    );

    const normalizedFilter = filter.trim().toLowerCase();
    const filteredUsers = users.filter(
        (user) =>
            !normalizedFilter ||
            user.username.toLowerCase().includes(normalizedFilter) ||
            user.display_name.toLowerCase().includes(normalizedFilter) ||
            user.primary_email.toLowerCase().includes(normalizedFilter),
    );

    return (
        <Stack spacing={2}>
            <Typography variant="h4">{translate("Users")}</Typography>
            <Alert severity="info">
                {translate(
                    "User changes require a recent password check. Telegram login remains active after reauthentication.",
                )}
            </Alert>
            <Stack direction={{ sm: "row", xs: "column" }} spacing={1}>
                <TextField
                    label={translate("Administrator password")}
                    type="password"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    size="small"
                />
                <Button variant="outlined" disabled={!password} onClick={() => unlock().catch(console.error)}>
                    {translate("Unlock changes")}
                </Button>
                <Button variant="contained" onClick={() => setCreateOpen(true)}>
                    {translate("Create user")}
                </Button>
            </Stack>
            <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { lg: "minmax(260px, 1fr) 2fr", xs: "1fr" } }}>
                <Card variant="outlined">
                    <CardContent>
                        <Typography variant="h6">{translate("Accounts")}</Typography>
                        <TextField
                            fullWidth
                            label={translate("Filter users")}
                            margin="dense"
                            size="small"
                            value={filter}
                            onChange={(event) => setFilter(event.target.value)}
                        />
                        {loading ? <Typography>{translate("Loading")}</Typography> : null}
                        <List>
                            {filteredUsers.map((user) => (
                                <ListItem disablePadding key={user.username}>
                                    <ListItemButton onClick={() => openUser(user.username).catch(console.error)}>
                                        <ListItemText
                                            primary={user.username}
                                            secondary={user.primary_email + " · " + user.status}
                                        />
                                    </ListItemButton>
                                </ListItem>
                            ))}
                        </List>
                    </CardContent>
                </Card>
                {selected ? (
                    <UserDetails
                        key={`${selected.username}:${selected.version}`}
                        details={selected}
                        currentUsername={currentUsername}
                        applyDetails={applyDetails}
                    />
                ) : null}
            </Box>
            <CreateUserDialog
                open={createOpen}
                close={() => setCreateOpen(false)}
                created={async (details) => {
                    setSelected(details);
                    setCreateOpen(false);
                    await loadUsers();
                }}
            />
        </Stack>
    );
};

interface UserDetailsProps {
    currentUsername: string;
    details: AdminUserDetails;
    applyDetails: (_operation: () => Promise<AdminUserDetails>) => Promise<void>;
}

const UserDetails = function ({ applyDetails, currentUsername, details }: UserDetailsProps) {
    const { t: translate } = useTranslation("settings");
    const { createErrorNotification, createSuccessNotification } = useNotifications();
    const [displayName, setDisplayName] = useState(details.display_name);
    const [status, setStatus] = useState<AdminUserSummary["status"]>(details.status);
    const [confirmation, setConfirmation] = useState("");
    const [newEmail, setNewEmail] = useState("");
    const [newEmailPrimary, setNewEmailPrimary] = useState(false);
    const [newGroup, setNewGroup] = useState("");
    const [setupLink, setSetupLink] = useState("");
    const [setupExpires, setSetupExpires] = useState("");
    const confirmed = confirmation === currentUsername;
    const selfDisableRequiresConfirmation = details.username === currentUsername && status === "disabled";
    const lastLoginUnlinkRequiresConfirmation =
        details.username === currentUsername && !details.password_enabled && details.identities.length === 1;

    const generateLink = useCallback(async () => {
        try {
            const link = await generateAdminUserSetupLink(details.username);
            setSetupLink(link.setup_url);
            setSetupExpires(new Date(link.expires_at).toLocaleString());
        } catch {
            createErrorNotification(translate("Failed to generate setup link"));
        }
    }, [createErrorNotification, details.username, translate]);

    const copyLink = useCallback(async () => {
        try {
            await navigator.clipboard.writeText(setupLink);
            createSuccessNotification(translate("Setup link copied"));
        } catch {
            createErrorNotification(translate("Failed to copy setup link"));
        }
    }, [createErrorNotification, createSuccessNotification, setupLink, translate]);

    return (
        <Card variant="outlined">
            <CardContent>
                <Stack spacing={2}>
                    <Stack direction="row" spacing={1} alignItems="center">
                        <Typography variant="h5">{details.username}</Typography>
                        <Chip label={details.status} color={details.status === "active" ? "success" : "default"} />
                        {details.password_enabled ? <Chip label={translate("Password enabled")} /> : null}
                    </Stack>
                    <TextField
                        label={translate("Display name")}
                        value={displayName}
                        onChange={(event) => setDisplayName(event.target.value)}
                    />
                    <TextField
                        label={translate("Status")}
                        select
                        value={status}
                        onChange={(event) => setStatus(event.target.value as AdminUserSummary["status"])}
                    >
                        <MenuItem value="active">{translate("Active")}</MenuItem>
                        <MenuItem value="disabled">{translate("Disabled")}</MenuItem>
                    </TextField>
                    <TextField
                        label={translate("Confirmation username")}
                        value={confirmation}
                        helperText={translate("Required for changes that can remove your own access")}
                        onChange={(event) => setConfirmation(event.target.value)}
                    />
                    <Button
                        variant="contained"
                        disabled={selfDisableRequiresConfirmation && !confirmed}
                        onClick={() =>
                            applyDetails(() =>
                                updateAdminUser(details.username, details.version, displayName, status, confirmation),
                            ).catch(console.error)
                        }
                    >
                        {translate("Save user")}
                    </Button>
                    <Divider />
                    <Typography variant="h6">{translate("Email addresses")}</Typography>
                    {details.emails.map((email) => (
                        <Stack
                            direction={{ sm: "row", xs: "column" }}
                            spacing={1}
                            key={email.email}
                            alignItems="center"
                        >
                            <Typography sx={{ flexGrow: 1 }}>{email.email}</Typography>
                            {email.primary ? <Chip label={translate("Primary")} size="small" /> : null}
                            <Button
                                aria-label={"Make primary " + email.email}
                                disabled={email.primary}
                                onClick={() =>
                                    applyDetails(() =>
                                        setAdminUserPrimaryEmail(details.username, email.email, details.version),
                                    ).catch(console.error)
                                }
                            >
                                {translate("Make primary")}
                            </Button>
                            <Button
                                aria-label={"Delete " + email.email}
                                color="error"
                                onClick={() =>
                                    applyDetails(() =>
                                        deleteAdminUserEmail(details.username, email.email, details.version),
                                    ).catch(console.error)
                                }
                            >
                                {translate("Delete")}
                            </Button>
                        </Stack>
                    ))}
                    <Stack direction={{ sm: "row", xs: "column" }} spacing={1}>
                        <TextField
                            label={translate("New email")}
                            value={newEmail}
                            onChange={(event) => setNewEmail(event.target.value)}
                            fullWidth
                        />
                        <FormControlLabel
                            control={
                                <Switch
                                    checked={newEmailPrimary}
                                    onChange={(event) => setNewEmailPrimary(event.target.checked)}
                                />
                            }
                            label={translate("Primary")}
                        />
                        <Button
                            disabled={!newEmail}
                            onClick={() =>
                                applyDetails(() =>
                                    addAdminUserEmail(details.username, newEmail, details.version, newEmailPrimary),
                                ).catch(console.error)
                            }
                        >
                            {translate("Add email")}
                        </Button>
                    </Stack>
                    <Divider />
                    <Typography variant="h6">{translate("Groups")}</Typography>
                    {details.groups.map((group) => (
                        <Stack direction="row" spacing={1} alignItems="center" key={group}>
                            <Typography sx={{ flexGrow: 1 }}>{group}</Typography>
                            <Button
                                aria-label={"Remove " + group}
                                color="error"
                                onClick={() =>
                                    applyDetails(async () => {
                                        const current = await getAdminGroup(group);
                                        await removeAdminGroupUser(
                                            group,
                                            details.username,
                                            current.version,
                                            confirmation,
                                        );
                                        return getAdminUser(details.username);
                                    }).catch(console.error)
                                }
                            >
                                {translate("Remove")}
                            </Button>
                        </Stack>
                    ))}
                    <Stack direction={{ sm: "row", xs: "column" }} spacing={1}>
                        <TextField
                            fullWidth
                            label={translate("New group")}
                            value={newGroup}
                            onChange={(event) => setNewGroup(event.target.value)}
                        />
                        <Button
                            disabled={!newGroup || details.groups.includes(newGroup)}
                            onClick={() =>
                                applyDetails(async () => {
                                    const current = await getAdminGroup(newGroup);
                                    await addAdminGroupUser(newGroup, details.username, current.version);
                                    return getAdminUser(details.username);
                                }).catch(console.error)
                            }
                        >
                            {translate("Add group")}
                        </Button>
                    </Stack>
                    <Divider />
                    <Typography variant="h6">{translate("Linked identities")}</Typography>
                    {details.identities.map((identity) => (
                        <Stack direction="row" spacing={1} alignItems="center" key={identity.provider}>
                            <Typography sx={{ flexGrow: 1 }}>
                                {identity.provider}: {identity.provider_username ?? identity.provider_user_id}
                            </Typography>
                            <Button
                                aria-label={"Unlink " + identity.provider}
                                color="error"
                                disabled={lastLoginUnlinkRequiresConfirmation && !confirmed}
                                onClick={() =>
                                    applyDetails(() =>
                                        unlinkAdminUserIdentity(
                                            details.username,
                                            identity.provider,
                                            details.version,
                                            confirmation,
                                        ),
                                    ).catch(console.error)
                                }
                            >
                                {translate("Unlink")}
                            </Button>
                        </Stack>
                    ))}
                    <Divider />
                    <Button variant="outlined" onClick={() => generateLink().catch(console.error)}>
                        {translate("Generate setup link")}
                    </Button>
                    {setupLink ? (
                        <Stack spacing={1}>
                            <TextField
                                label={translate("One-time setup link")}
                                value={setupLink}
                                slotProps={{ htmlInput: { readOnly: true } }}
                            />
                            <Typography>
                                {translate("Expires")}: {setupExpires}
                            </Typography>
                            <Button onClick={() => copyLink().catch(console.error)}>{translate("Copy link")}</Button>
                        </Stack>
                    ) : null}
                </Stack>
            </CardContent>
        </Card>
    );
};

interface CreateUserDialogProps {
    open: boolean;
    close: () => void;
    created: (_details: AdminUserDetails) => Promise<void>;
}

const CreateUserDialog = function ({ close, created, open }: CreateUserDialogProps) {
    const { t: translate } = useTranslation("settings");
    const { createErrorNotification } = useNotifications();
    const [form, setForm] = useState<AdminUserCreate>({ display_name: "", email: "", groups: [], username: "" });
    const [group, setGroup] = useState("");
    const [groups, setGroups] = useState<string[]>([]);

    const submit = useCallback(async () => {
        try {
            await created(await createAdminUser({ ...form, groups }));
            setForm({ display_name: "", email: "", groups: [], username: "" });
            setGroup("");
            setGroups([]);
        } catch {
            createErrorNotification(translate("Failed to create user; reauthenticate and try again"));
        }
    }, [createErrorNotification, created, form, groups, translate]);

    return (
        <Dialog open={open} onClose={close} fullWidth>
            <DialogTitle>{translate("Create user")}</DialogTitle>
            <DialogContent>
                <Stack spacing={2} sx={{ pt: 1 }}>
                    <TextField
                        label={translate("Username")}
                        value={form.username}
                        onChange={(event) => setForm({ ...form, username: event.target.value })}
                    />
                    <TextField
                        label={translate("Display name")}
                        value={form.display_name}
                        onChange={(event) => setForm({ ...form, display_name: event.target.value })}
                    />
                    <TextField
                        label={translate("Email")}
                        value={form.email}
                        onChange={(event) => setForm({ ...form, email: event.target.value })}
                    />
                    <Stack direction="row" spacing={1}>
                        <TextField
                            label={translate("New group")}
                            value={group}
                            onChange={(event) => setGroup(event.target.value)}
                            fullWidth
                        />
                        <Button
                            disabled={!group || groups.includes(group)}
                            onClick={() => {
                                setGroups([...groups, group]);
                                setGroup("");
                            }}
                        >
                            {translate("Add group")}
                        </Button>
                    </Stack>
                    <Stack direction="row" spacing={1} useFlexGap flexWrap="wrap">
                        {groups.map((value) => (
                            <Chip
                                key={value}
                                label={value}
                                onDelete={() => setGroups(groups.filter((item) => item !== value))}
                            />
                        ))}
                    </Stack>
                </Stack>
            </DialogContent>
            <DialogActions>
                <Button onClick={close}>{translate("Cancel")}</Button>
                <Button variant="contained" onClick={() => submit().catch(console.error)}>
                    {translate("Save new user")}
                </Button>
            </DialogActions>
        </Dialog>
    );
};

export default UsersView;
