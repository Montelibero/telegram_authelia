import { useCallback, useEffect, useState } from "react";

import {
    Box,
    Button,
    Card,
    CardContent,
    Chip,
    List,
    ListItem,
    ListItemButton,
    ListItemText,
    Stack,
    Tab,
    Tabs,
    TextField,
    Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { useNotifications } from "@contexts/NotificationsContext";
import {
    AdminRegistration,
    approveAdminRegistration,
    getAdminRegistration,
    getAdminRegistrations,
    rejectAdminRegistration,
} from "@services/Admin";
import { isAdminMutationAuthenticationError, useAdminMutationLock } from "@views/Settings/Admin/useAdminMutationLock";

type RegistrationFilter = "all" | AdminRegistration["status"];

const providerIdentityLabel = (registration: AdminRegistration) =>
    registration.provider_username ? `@${registration.provider_username}` : registration.provider_user_id;

const providerIDLabel = (registration: AdminRegistration) =>
    registration.provider === "telegram"
        ? `Telegram ID: ${registration.provider_user_id}`
        : registration.provider_user_id;

const PendingView = function () {
    const { t: translate } = useTranslation("settings");
    const { createErrorNotification, createSuccessNotification } = useNotifications();
    const [filter, setFilter] = useState<RegistrationFilter>("pending");
    const [registrations, setRegistrations] = useState<AdminRegistration[]>([]);
    const [selected, setSelected] = useState<AdminRegistration>();
    const mutationLock = useAdminMutationLock();

    const loadRegistrations = useCallback(async () => {
        try {
            setRegistrations(await getAdminRegistrations(filter === "all" ? undefined : filter));
        } catch {
            createErrorNotification(translate("Failed to load registrations"));
        }
    }, [createErrorNotification, filter, translate]);

    useEffect(() => {
        loadRegistrations().catch(console.error);
    }, [loadRegistrations]);

    const openRegistration = useCallback(
        async (id: number) => {
            try {
                setSelected(await getAdminRegistration(id));
            } catch {
                createErrorNotification(translate("Failed to load registration"));
            }
        },
        [createErrorNotification, translate],
    );

    const resolve = useCallback(
        async (operation: () => Promise<unknown>) => {
            try {
                await operation();
                setSelected(undefined);
                await loadRegistrations();
                createSuccessNotification(translate("Registration resolved"));
            } catch (error) {
                if (isAdminMutationAuthenticationError(error)) {
                    mutationLock.lock();
                    createErrorNotification(translate("Reauthenticate to make administrator changes"));
                    return;
                }
                if ((error as { response?: { status?: number } }).response?.status === 409 && selected) {
                    try {
                        setSelected(await getAdminRegistration(selected.id));
                        await loadRegistrations();
                        createErrorNotification(
                            translate("Registration changed elsewhere; the latest version has been loaded"),
                        );
                        return;
                    } catch {
                        // Fall through when refreshing the conflicting record also fails.
                    }
                }
                createErrorNotification(translate("Registration update failed; reauthenticate and try again"));
            }
        },
        [createErrorNotification, createSuccessNotification, loadRegistrations, mutationLock, selected, translate],
    );

    return (
        <Stack spacing={2}>
            <Typography variant="h4">{translate("Pending registrations")}</Typography>
            {mutationLock.controls}
            <Stack direction={{ sm: "row", xs: "column" }} spacing={1}>
                <Tabs
                    aria-label={translate("Status filter")}
                    value={filter}
                    onChange={(_event, value: RegistrationFilter) => {
                        setSelected(undefined);
                        setFilter(value);
                    }}
                    variant="scrollable"
                    scrollButtons="auto"
                >
                    <Tab value="pending" label={translate("Pending")} />
                    <Tab value="approved" label={translate("Approved")} />
                    <Tab value="rejected" label={translate("Rejected")} />
                    <Tab value="all" label={translate("All")} />
                </Tabs>
            </Stack>
            <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { lg: "minmax(260px, 1fr) 2fr", xs: "1fr" } }}>
                <Card variant="outlined">
                    <CardContent>
                        <List>
                            {registrations.map((registration) => (
                                <ListItem disablePadding key={registration.id}>
                                    <ListItemButton
                                        onClick={() => openRegistration(registration.id).catch(console.error)}
                                    >
                                        <ListItemText
                                            primary={providerIdentityLabel(registration)}
                                            secondary={
                                                <>
                                                    <span>{providerIDLabel(registration)}</span>
                                                    <br />
                                                    <span>{registration.proposed_email ?? registration.status}</span>
                                                </>
                                            }
                                        />
                                    </ListItemButton>
                                </ListItem>
                            ))}
                        </List>
                    </CardContent>
                </Card>
                {selected ? (
                    <RegistrationDetails
                        key={selected.id + ":" + selected.version}
                        registration={selected}
                        resolve={resolve}
                        mutationsUnlocked={mutationLock.unlocked}
                    />
                ) : null}
            </Box>
        </Stack>
    );
};

interface RegistrationDetailsProps {
    registration: AdminRegistration;
    resolve: (_operation: () => Promise<unknown>) => Promise<void>;
    mutationsUnlocked: boolean;
}

const RegistrationDetails = function ({ mutationsUnlocked, registration, resolve }: RegistrationDetailsProps) {
    const { t: translate } = useTranslation("settings");
    const [username, setUsername] = useState(registration.proposed_username ?? "");
    const [displayName, setDisplayName] = useState(registration.display_name ?? "");
    const [email, setEmail] = useState(registration.proposed_email ?? "");
    const [group, setGroup] = useState("");
    const [groups, setGroups] = useState<string[]>([]);

    return (
        <Card variant="outlined">
            <CardContent>
                <fieldset disabled={!mutationsUnlocked} style={{ border: 0, margin: 0, padding: 0 }}>
                    <Stack spacing={2}>
                        <Stack direction="row" spacing={1} alignItems="center">
                            <Typography variant="h5">
                                {registration.provider}: {providerIdentityLabel(registration)}
                            </Typography>
                            <Chip label={registration.status} />
                        </Stack>
                        <Typography color="text.secondary">{providerIDLabel(registration)}</Typography>
                        <TextField
                            label={translate("Username")}
                            value={username}
                            onChange={(e) => setUsername(e.target.value)}
                        />
                        <TextField
                            label={translate("Display name")}
                            value={displayName}
                            onChange={(e) => setDisplayName(e.target.value)}
                        />
                        <TextField
                            label={translate("Email")}
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
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
                        {registration.status === "pending" ? (
                            <Stack direction="row" spacing={1}>
                                <Button
                                    variant="contained"
                                    onClick={() =>
                                        resolve(() =>
                                            approveAdminRegistration({
                                                display_name: displayName,
                                                email,
                                                expected_version: registration.version,
                                                groups,
                                                id: registration.id,
                                                username,
                                            }),
                                        ).catch(console.error)
                                    }
                                >
                                    {translate("Approve")}
                                </Button>
                                <Button
                                    color="error"
                                    variant="outlined"
                                    onClick={() =>
                                        resolve(() =>
                                            rejectAdminRegistration(registration.id, registration.version),
                                        ).catch(console.error)
                                    }
                                >
                                    {translate("Reject")}
                                </Button>
                            </Stack>
                        ) : null}
                    </Stack>
                </fieldset>
            </CardContent>
        </Card>
    );
};

export default PendingView;
