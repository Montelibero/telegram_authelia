import { useCallback, useEffect, useState } from "react";

import {
    Alert,
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
    TextField,
    Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { useNotifications } from "@contexts/NotificationsContext";
import {
    AdminGroupDetails,
    AdminGroupWarning,
    addAdminGroupManager,
    addAdminGroupUser,
    createAdminGroup,
    deleteAdminGroup,
    getAdminGroup,
    getAdminGroups,
    removeAdminGroupManager,
    removeAdminGroupUser,
    renameAdminGroup,
} from "@services/Admin";
import { isAdminMutationAuthenticationError, useAdminMutationLock } from "@views/Settings/Admin/useAdminMutationLock";

interface GroupsViewProps {
    currentUsername: string;
    fullAdministrator?: boolean;
}

const GroupsView = function ({ currentUsername, fullAdministrator = true }: GroupsViewProps) {
    const { t: translate } = useTranslation("settings");
    const { createErrorNotification, createSuccessNotification } = useNotifications();
    const [groups, setGroups] = useState<Awaited<ReturnType<typeof getAdminGroups>>>([]);
    const [selected, setSelected] = useState<AdminGroupDetails>();
    const [newGroupName, setNewGroupName] = useState("");
    const [warning, setWarning] = useState<{ affectedUsers: string[]; message: string }>();
    const mutationLock = useAdminMutationLock();

    const loadGroups = useCallback(async () => {
        try {
            setGroups(await getAdminGroups());
        } catch {
            createErrorNotification(translate("Failed to load groups"));
        }
    }, [createErrorNotification, translate]);

    useEffect(() => {
        loadGroups().catch(console.error);
    }, [loadGroups]);

    const openGroup = useCallback(
        async (name: string) => {
            try {
                setSelected(await getAdminGroup(name));
                setWarning(undefined);
            } catch {
                createErrorNotification(translate("Failed to load group"));
            }
        },
        [createErrorNotification, translate],
    );

    const create = useCallback(async () => {
        try {
            setSelected(await createAdminGroup(newGroupName));
            setNewGroupName("");
            await loadGroups();
            createSuccessNotification(translate("Group created"));
        } catch (error) {
            if (isAdminMutationAuthenticationError(error)) mutationLock.lock();
            createErrorNotification(translate("Group creation failed; reauthenticate and try again"));
        }
    }, [createErrorNotification, createSuccessNotification, loadGroups, mutationLock, newGroupName, translate]);

    const applyGroup = useCallback(
        async (operation: () => Promise<AdminGroupDetails | AdminGroupWarning>) => {
            const selectedName = selected?.name;
            try {
                const result = await operation();
                if ("external_acl_not_updated" in result) {
                    setWarning(
                        result.external_acl_not_updated
                            ? {
                                  affectedUsers: result.affected_users,
                                  message: translate("External ACL configuration was not changed"),
                              }
                            : undefined,
                    );
                    setSelected(result.group?.name ? result.group : undefined);
                } else {
                    setWarning(undefined);
                    setSelected(result);
                }
                await loadGroups();
                createSuccessNotification(translate("Group updated"));
            } catch (error) {
                if (isAdminMutationAuthenticationError(error)) {
                    mutationLock.lock();
                    createErrorNotification(translate("Reauthenticate to make administrator changes"));
                    return;
                }
                if ((error as { response?: { status?: number } }).response?.status === 409 && selectedName) {
                    try {
                        setSelected(await getAdminGroup(selectedName));
                        await loadGroups();
                        createErrorNotification(
                            translate("Group changed elsewhere; the latest version has been loaded"),
                        );
                        return;
                    } catch {
                        /* Fall through when refreshing the conflicting group also fails. */
                    }
                }
                createErrorNotification(translate("Group update failed; reauthenticate or reload and try again"));
            }
        },
        [createErrorNotification, createSuccessNotification, loadGroups, mutationLock, selected?.name, translate],
    );

    return (
        <Stack spacing={2}>
            <Typography variant="h4">{translate("Groups")}</Typography>
            {mutationLock.controls}
            {warning ? (
                <Alert severity="warning">
                    {warning.message}. {translate("Affected users")}:{" "}
                    {warning.affectedUsers.join(", ") || translate("none")}.
                </Alert>
            ) : null}
            {fullAdministrator ? (
                <Stack direction={{ sm: "row", xs: "column" }} spacing={1}>
                    <TextField
                        disabled={!mutationLock.unlocked}
                        label={translate("New group name")}
                        value={newGroupName}
                        onChange={(event) => setNewGroupName(event.target.value)}
                        size="small"
                    />
                    <Button
                        variant="contained"
                        disabled={!mutationLock.unlocked || !newGroupName}
                        onClick={() => create().catch(console.error)}
                    >
                        {translate("Create group")}
                    </Button>
                </Stack>
            ) : null}
            <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { lg: "minmax(260px, 1fr) 2fr", xs: "1fr" } }}>
                <Card variant="outlined">
                    <CardContent>
                        <List>
                            {groups.map((group) => (
                                <ListItem disablePadding key={group.name}>
                                    <ListItemButton onClick={() => openGroup(group.name).catch(console.error)}>
                                        <ListItemText
                                            primary={group.name}
                                            secondary={`${group.user_count} users${group.managed ? ` · ${translate("Managed application group")}` : ""}`}
                                        />
                                    </ListItemButton>
                                </ListItem>
                            ))}
                        </List>
                    </CardContent>
                </Card>
                {selected ? (
                    <GroupDetails
                        key={`${selected.name}:${selected.version}`}
                        currentUsername={currentUsername}
                        details={selected}
                        fullAdministrator={fullAdministrator}
                        applyGroup={applyGroup}
                        mutationsUnlocked={mutationLock.unlocked}
                    />
                ) : null}
            </Box>
        </Stack>
    );
};

interface GroupDetailsProps {
    applyGroup: (_operation: () => Promise<AdminGroupDetails | AdminGroupWarning>) => Promise<void>;
    currentUsername: string;
    details: AdminGroupDetails;
    fullAdministrator: boolean;
    mutationsUnlocked: boolean;
}

const GroupDetails = function ({
    applyGroup,
    currentUsername,
    details,
    fullAdministrator,
    mutationsUnlocked,
}: GroupDetailsProps) {
    const { t: translate } = useTranslation("settings");
    const [name, setName] = useState(details.name);
    const [confirmation, setConfirmation] = useState("");
    const [usernameToAdd, setUsernameToAdd] = useState("");
    const [managerToAdd, setManagerToAdd] = useState("");
    const affectsCurrentAdministrator = details.users.includes(currentUsername);
    const confirmed = confirmation === currentUsername;

    return (
        <Card variant="outlined">
            <CardContent>
                <fieldset disabled={!mutationsUnlocked} style={{ border: 0, margin: 0, padding: 0 }}>
                    <Stack spacing={2}>
                        {details.managed ? <Chip label={translate("Managed application group")} /> : null}
                        {fullAdministrator ? (
                            <>
                                <TextField
                                    disabled={details.managed}
                                    label={translate("Group name")}
                                    value={name}
                                    onChange={(event) => setName(event.target.value)}
                                />
                                <TextField
                                    label={translate("Confirmation username")}
                                    value={confirmation}
                                    helperText={translate("Required for changes that can remove your own access")}
                                    onChange={(event) => setConfirmation(event.target.value)}
                                />
                                <Stack direction={{ sm: "row", xs: "column" }} spacing={1}>
                                    <Button
                                        variant="contained"
                                        disabled={details.managed || (affectsCurrentAdministrator && !confirmed)}
                                        onClick={() =>
                                            applyGroup(() =>
                                                renameAdminGroup(
                                                    details.name,
                                                    name,
                                                    details.version,
                                                    affectsCurrentAdministrator ? confirmation : "",
                                                ),
                                            ).catch(console.error)
                                        }
                                    >
                                        {translate("Rename group")}
                                    </Button>
                                    <Button
                                        color="error"
                                        variant="outlined"
                                        disabled={details.managed || (affectsCurrentAdministrator && !confirmed)}
                                        onClick={() =>
                                            applyGroup(() =>
                                                deleteAdminGroup(
                                                    details.name,
                                                    details.version,
                                                    affectsCurrentAdministrator ? confirmation : "",
                                                ),
                                            ).catch(console.error)
                                        }
                                    >
                                        {translate("Delete group")}
                                    </Button>
                                </Stack>
                            </>
                        ) : null}
                        <Stack direction={{ sm: "row", xs: "column" }} spacing={1}>
                            <TextField
                                label={translate("Username to add")}
                                value={usernameToAdd}
                                onChange={(event) => setUsernameToAdd(event.target.value)}
                                fullWidth
                            />
                            <Button
                                disabled={!usernameToAdd}
                                onClick={() =>
                                    applyGroup(() =>
                                        addAdminGroupUser(details.name, usernameToAdd, details.version),
                                    ).catch(console.error)
                                }
                            >
                                {translate("Add member")}
                            </Button>
                        </Stack>
                        <List>
                            {details.users.map((username) => {
                                const removesCurrentAdministrator = username === currentUsername;
                                return (
                                    <ListItem
                                        key={username}
                                        secondaryAction={
                                            <Button
                                                aria-label={`Remove ${username}`}
                                                color="error"
                                                disabled={removesCurrentAdministrator && !confirmed}
                                                onClick={() =>
                                                    applyGroup(() =>
                                                        removeAdminGroupUser(
                                                            details.name,
                                                            username,
                                                            details.version,
                                                            removesCurrentAdministrator ? confirmation : "",
                                                        ),
                                                    ).catch(console.error)
                                                }
                                            >
                                                {translate("Remove")}
                                            </Button>
                                        }
                                    >
                                        <ListItemText primary={username} />
                                    </ListItem>
                                );
                            })}
                        </List>
                        {fullAdministrator ? (
                            <>
                                <Typography variant="h6">{translate("Managers")}</Typography>
                                <Stack direction={{ sm: "row", xs: "column" }} spacing={1}>
                                    <TextField
                                        label={translate("Manager username")}
                                        value={managerToAdd}
                                        onChange={(event) => setManagerToAdd(event.target.value)}
                                        fullWidth
                                    />
                                    <Button
                                        disabled={!managerToAdd || (details.managers ?? []).includes(managerToAdd)}
                                        onClick={() =>
                                            applyGroup(() =>
                                                addAdminGroupManager(details.name, managerToAdd, details.version),
                                            ).catch(console.error)
                                        }
                                    >
                                        {translate("Add manager")}
                                    </Button>
                                </Stack>
                                <List>
                                    {(details.managers ?? []).map((username) => (
                                        <ListItem
                                            key={username}
                                            secondaryAction={
                                                <Button
                                                    aria-label={`Remove manager ${username}`}
                                                    color="error"
                                                    onClick={() =>
                                                        applyGroup(() =>
                                                            removeAdminGroupManager(
                                                                details.name,
                                                                username,
                                                                details.version,
                                                            ),
                                                        ).catch(console.error)
                                                    }
                                                >
                                                    {translate("Remove")}
                                                </Button>
                                            }
                                        >
                                            <ListItemText primary={username} />
                                        </ListItem>
                                    ))}
                                </List>
                            </>
                        ) : null}
                    </Stack>
                </fieldset>
            </CardContent>
        </Card>
    );
};

export default GroupsView;
