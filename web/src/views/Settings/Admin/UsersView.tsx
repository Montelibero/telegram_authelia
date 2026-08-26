import { useCallback, useEffect, useState } from "react";

import {
    Box,
    Button,
    Card,
    CardContent,
    Checkbox,
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
    TextField,
    Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { useNotifications } from "@contexts/NotificationsContext";
import {
    AdminUserCreate,
    AdminUserDetails,
    AdminUserSetupLink,
    AdminUserSummary,
    addAdminGroupUser,
    addAdminUserEmail,
    createAdminUser,
    deleteAdminUserEmail,
    generateAdminUserSetupLink,
    getAdminApplications,
    getAdminGroup,
    getAdminUser,
    getAdminUsers,
    linkAdminUserTelegram,
    removeAdminGroupUser,
    setAdminUserPrimaryEmail,
    unlinkAdminUserIdentity,
    updateAdminUser,
} from "@services/Admin";
import { isAdminMutationAuthenticationError, useAdminMutationLock } from "@views/Settings/Admin/useAdminMutationLock";

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
    const [filter, setFilter] = useState("");
    const [permissionGroups, setPermissionGroups] = useState<string[]>([]);
    const mutationLock = useAdminMutationLock();

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

    useEffect(() => {
        getAdminApplications()
            .then((permissions) => setPermissionGroups(permissions.map((permission) => permission.group)))
            .catch(() => createErrorNotification(translate("Failed to load permissions")));
    }, [createErrorNotification, translate]);

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

    const applyDetails = useCallback(
        async (operation: () => Promise<AdminUserDetails>) => {
            try {
                const details = await operation();
                setSelected(details);
                await loadUsers();
                createSuccessNotification(translate("User updated"));
            } catch (error) {
                if (isAdminMutationAuthenticationError(error)) {
                    mutationLock.lock();
                    createErrorNotification(translate("Reauthenticate to make administrator changes"));
                    return;
                }
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
        [createErrorNotification, createSuccessNotification, loadUsers, mutationLock, selected, translate],
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
            {mutationLock.controls}
            <Stack direction={{ sm: "row", xs: "column" }} spacing={1}>
                <Button variant="contained" disabled={!mutationLock.unlocked} onClick={() => setCreateOpen(true)}>
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
                                            primary={
                                                user.provisioning_status === "awaiting_first_login"
                                                    ? `telegram: ${user.telegram_id}`
                                                    : user.username
                                            }
                                            secondary={
                                                (user.primary_email.endsWith("@pending.invalid")
                                                    ? ""
                                                    : user.primary_email + " · ") +
                                                (user.provisioning_status || user.status)
                                            }
                                        />
                                    </ListItemButton>
                                </ListItem>
                            ))}
                        </List>
                    </CardContent>
                </Card>
                {selected ? (
                    <UserDetails
                        availableGroups={permissionGroups}
                        key={`${selected.username}:${selected.version}`}
                        details={selected}
                        currentUsername={currentUsername}
                        lockMutations={mutationLock.lock}
                        mutationsUnlocked={mutationLock.unlocked}
                        applyDetails={applyDetails}
                    />
                ) : null}
            </Box>
            <CreateUserDialog
                availableGroups={permissionGroups}
                open={createOpen}
                close={() => setCreateOpen(false)}
                lockMutations={mutationLock.lock}
                created={async (details) => {
                    setSelected(details);
                    await loadUsers();
                }}
            />
        </Stack>
    );
};

interface UserDetailsProps {
    availableGroups: string[];
    currentUsername: string;
    details: AdminUserDetails;
    lockMutations: () => void;
    mutationsUnlocked: boolean;
    applyDetails: (_operation: () => Promise<AdminUserDetails>) => Promise<void>;
}

const UserDetails = function ({
    applyDetails,
    availableGroups,
    currentUsername,
    details,
    lockMutations,
    mutationsUnlocked,
}: UserDetailsProps) {
    const { t: translate } = useTranslation("settings");
    const { createErrorNotification, createSuccessNotification } = useNotifications();
    const [displayName, setDisplayName] = useState(details.display_name);
    const [status, setStatus] = useState<AdminUserSummary["status"]>(details.status);
    const [confirmation, setConfirmation] = useState("");
    const [newEmail, setNewEmail] = useState("");
    const [telegramID, setTelegramID] = useState(details.telegram_id || "");
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
        } catch (error) {
            if (isAdminMutationAuthenticationError(error)) lockMutations();
            createErrorNotification(translate("Failed to generate setup link"));
        }
    }, [createErrorNotification, details.username, lockMutations, translate]);

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
                <fieldset disabled={!mutationsUnlocked} style={{ border: 0, margin: 0, padding: 0 }}>
                    <Stack spacing={2}>
                        <Stack direction="row" spacing={1} alignItems="center">
                            <Typography variant="h5">
                                {details.provisioning_status === "awaiting_first_login"
                                    ? `telegram: ${details.telegram_id}`
                                    : details.username}
                            </Typography>
                            <Chip label={details.status} color={details.status === "active" ? "success" : "default"} />
                            {details.provisioning_status ? <Chip label={details.provisioning_status} /> : null}
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
                                    updateAdminUser(
                                        details.username,
                                        details.version,
                                        displayName,
                                        status,
                                        confirmation,
                                    ),
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
                            <Button
                                disabled={!newEmail}
                                onClick={() =>
                                    applyDetails(() =>
                                        addAdminUserEmail(details.username, newEmail, details.version, false),
                                    ).catch(console.error)
                                }
                            >
                                {translate("Add email")}
                            </Button>
                        </Stack>
                        <Divider />
                        <Typography variant="h6">{translate("Groups")}</Typography>
                        <Stack>
                            {availableGroups.map((group) => (
                                <FormControlLabel
                                    key={group}
                                    label={group}
                                    control={
                                        <Checkbox
                                            checked={details.groups.includes(group)}
                                            onChange={(_, checked) =>
                                                applyDetails(async () => {
                                                    const current = await getAdminGroup(group);
                                                    if (checked) {
                                                        await addAdminGroupUser(
                                                            group,
                                                            details.username,
                                                            current.version,
                                                        );
                                                    } else {
                                                        await removeAdminGroupUser(
                                                            group,
                                                            details.username,
                                                            current.version,
                                                            confirmation,
                                                        );
                                                    }
                                                    return getAdminUser(details.username);
                                                }).catch(console.error)
                                            }
                                        />
                                    }
                                />
                            ))}
                        </Stack>
                        <Divider />
                        <Typography variant="h6">{translate("Linked identities")}</Typography>
                        {details.identities.map((identity) => (
                            <Stack direction="row" spacing={1} alignItems="center" key={identity.provider}>
                                <Typography sx={{ flexGrow: 1 }}>
                                    {identity.provider}: {identity.provider_username || identity.provider_user_id}
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
                        <Stack direction={{ sm: "row", xs: "column" }} spacing={1}>
                            <TextField
                                fullWidth
                                label={translate("Telegram ID")}
                                value={telegramID}
                                slotProps={{ htmlInput: { inputMode: "numeric" } }}
                                onChange={(event) => setTelegramID(event.target.value)}
                            />
                            <Button
                                disabled={!telegramID.trim() || telegramID.trim() === details.telegram_id}
                                onClick={() =>
                                    applyDetails(() =>
                                        linkAdminUserTelegram(details.username, telegramID, details.version),
                                    ).catch(console.error)
                                }
                            >
                                {translate(details.telegram_id ? "Replace Telegram" : "Link Telegram")}
                            </Button>
                        </Stack>
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
                                <Button onClick={() => copyLink().catch(console.error)}>
                                    {translate("Copy link")}
                                </Button>
                            </Stack>
                        ) : null}
                    </Stack>
                </fieldset>
            </CardContent>
        </Card>
    );
};

interface CreateUserDialogProps {
    availableGroups: string[];
    open: boolean;
    close: () => void;
    created: (_details: AdminUserDetails) => Promise<void>;
    lockMutations: () => void;
}

const CreateUserDialog = function ({ availableGroups, close, created, lockMutations, open }: CreateUserDialogProps) {
    const { t: translate } = useTranslation("settings");
    const { createErrorNotification, createSuccessNotification } = useNotifications();
    const [form, setForm] = useState<AdminUserCreate>({
        display_name: "",
        email: "",
        groups: [],
        telegram_id: "",
        username: "",
    });
    const [groups, setGroups] = useState<string[]>([]);
    const [setupLink, setSetupLink] = useState<AdminUserSetupLink>();

    const submit = useCallback(async () => {
        try {
            const details = await createAdminUser({ ...form, groups });
            await created(details);
            if (!form.telegram_id?.trim()) {
                setSetupLink(await generateAdminUserSetupLink(details.username));
                return;
            }
            setForm({ display_name: "", email: "", groups: [], telegram_id: "", username: "" });
            setGroups([]);
            close();
        } catch (error) {
            if (isAdminMutationAuthenticationError(error)) lockMutations();
            createErrorNotification(translate("Failed to create user; reauthenticate and try again"));
        }
    }, [close, createErrorNotification, created, form, groups, lockMutations, translate]);

    const copySetupLink = useCallback(async () => {
        if (!setupLink) return;
        try {
            await navigator.clipboard.writeText(setupLink.setup_url);
            createSuccessNotification(translate("Setup link copied"));
        } catch {
            createErrorNotification(translate("Failed to copy setup link"));
        }
    }, [createErrorNotification, createSuccessNotification, setupLink, translate]);

    const handleClose = useCallback(() => {
        setForm({ display_name: "", email: "", groups: [], telegram_id: "", username: "" });
        setGroups([]);
        setSetupLink(undefined);
        close();
    }, [close]);

    return (
        <Dialog open={open} onClose={handleClose} fullWidth>
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
                    <TextField
                        label={translate("Telegram ID")}
                        value={form.telegram_id}
                        slotProps={{ htmlInput: { inputMode: "numeric" } }}
                        onChange={(event) => setForm({ ...form, telegram_id: event.target.value })}
                    />
                    <Typography variant="subtitle1">{translate("Groups")}</Typography>
                    <Stack>
                        {availableGroups.map((group) => (
                            <FormControlLabel
                                key={group}
                                label={group}
                                control={
                                    <Checkbox
                                        checked={groups.includes(group)}
                                        onChange={(_, checked) =>
                                            setGroups(
                                                checked
                                                    ? [...groups, group]
                                                    : groups.filter((value) => value !== group),
                                            )
                                        }
                                    />
                                }
                            />
                        ))}
                    </Stack>
                    {setupLink ? (
                        <Stack spacing={1}>
                            <TextField
                                label={translate("One-time setup link")}
                                value={setupLink.setup_url}
                                slotProps={{ htmlInput: { readOnly: true } }}
                            />
                            <Typography>
                                {translate("Expires")}: {new Date(setupLink.expires_at).toLocaleString()}
                            </Typography>
                            <Button onClick={() => copySetupLink().catch(console.error)}>
                                {translate("Copy link")}
                            </Button>
                        </Stack>
                    ) : null}
                </Stack>
            </DialogContent>
            <DialogActions>
                <Button onClick={handleClose}>{translate("Cancel")}</Button>
                <Button
                    variant="contained"
                    disabled={Boolean(setupLink) || (!form.email.trim() && !form.telegram_id?.trim())}
                    onClick={() => submit().catch(console.error)}
                >
                    {translate("Save new user")}
                </Button>
            </DialogActions>
        </Dialog>
    );
};

export default UsersView;
