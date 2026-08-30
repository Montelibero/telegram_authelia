import { useCallback, useEffect, useMemo, useState } from "react";

import {
    Alert,
    Box,
    Checkbox,
    Chip,
    CircularProgress,
    Stack,
    Table,
    TableBody,
    TableCell,
    TableContainer,
    TableHead,
    TableRow,
    TextField,
    Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { useNotifications } from "@contexts/NotificationsContext";
import {
    AdminApplication,
    AdminApplicationUser,
    getAdminApplications,
    grantAdminApplicationUser,
    revokeAdminApplicationUser,
} from "@services/Admin";
import { isAdminMutationAuthenticationError, useAdminMutationLock } from "@views/Settings/Admin/useAdminMutationLock";

const PermissionsView = function () {
    const { t: translate } = useTranslation("settings");
    const { createErrorNotification, createSuccessNotification } = useNotifications();
    const [applications, setApplications] = useState<AdminApplication[]>([]);
    const [loading, setLoading] = useState(true);
    const [busy, setBusy] = useState("");
    const [userFilter, setUserFilter] = useState("");
    const [applicationFilter, setApplicationFilter] = useState("");
    const mutationLock = useAdminMutationLock();

    const loadApplications = useCallback(async () => {
        setLoading(true);
        try {
            setApplications(await getAdminApplications());
        } catch {
            createErrorNotification(translate("Failed to load permissions"));
        } finally {
            setLoading(false);
        }
    }, [createErrorNotification, translate]);

    useEffect(() => {
        loadApplications().catch(console.error);
    }, [loadApplications]);

    const mutate = useCallback(
        async (application: AdminApplication, user: AdminApplicationUser) => {
            const key = `${application.slug}:${user.username}`;
            setBusy(key);
            try {
                const updated = user.granted
                    ? await revokeAdminApplicationUser(application.slug, user.username, application.group_version)
                    : await grantAdminApplicationUser(application.slug, user.username, application.group_version);
                setApplications(updated);
                createSuccessNotification(translate("Permission updated"));
            } catch (error) {
                if (isAdminMutationAuthenticationError(error)) {
                    mutationLock.lock();
                    createErrorNotification(translate("Reauthenticate to make administrator changes"));
                    return;
                }
                if ((error as { response?: { status?: number } }).response?.status === 409) {
                    try {
                        setApplications(await getAdminApplications());
                        createErrorNotification(
                            translate("Permissions changed elsewhere; the latest version has been loaded"),
                        );
                        return;
                    } catch {
                        /* Fall through when refreshing the conflicting matrix also fails. */
                    }
                }
                createErrorNotification(translate("Permission update failed; reauthenticate or reload and try again"));
            } finally {
                setBusy("");
            }
        },
        [createErrorNotification, createSuccessNotification, mutationLock, translate],
    );

    const visibleApplications = useMemo(() => {
        const filter = applicationFilter.trim().toLocaleLowerCase();
        if (!filter) return applications;
        return applications.filter((application) =>
            [application.name, application.slug, application.domain, application.group].some((value) =>
                value.toLocaleLowerCase().includes(filter),
            ),
        );
    }, [applicationFilter, applications]);

    const users = useMemo(() => {
        const unique = new Map<string, AdminApplicationUser>();
        for (const application of applications) {
            for (const user of application.users) unique.set(user.username, user);
        }
        const filter = userFilter.trim().toLocaleLowerCase();
        return [...unique.values()].filter(
            (user) =>
                !filter ||
                [user.username, user.display_name, user.primary_email].some((value) =>
                    value.toLocaleLowerCase().includes(filter),
                ),
        );
    }, [applications, userFilter]);

    return (
        <Stack spacing={2} sx={{ p: { sm: 0, xs: 2 } }}>
            <Typography component="h1" variant="h4">
                {translate("Permissions")}
            </Typography>
            {mutationLock.controls}
            <Stack direction={{ sm: "row", xs: "column" }} spacing={1}>
                <TextField
                    label={translate("Filter users")}
                    value={userFilter}
                    onChange={(event) => setUserFilter(event.target.value)}
                    size="small"
                />
                <TextField
                    label={translate("Filter applications")}
                    value={applicationFilter}
                    onChange={(event) => setApplicationFilter(event.target.value)}
                    size="small"
                />
            </Stack>
            {loading ? (
                <Box sx={{ display: "flex", justifyContent: "center", p: 4 }}>
                    <CircularProgress />
                </Box>
            ) : applications.length === 0 ? (
                <Alert severity="info">{translate("No applications are configured")}</Alert>
            ) : (
                <TableContainer sx={{ maxWidth: "100%", overflowX: "auto" }}>
                    <Table size="small" sx={{ minWidth: 640 }}>
                        <TableHead>
                            <TableRow>
                                <TableCell>{translate("User")}</TableCell>
                                {visibleApplications.map((application) => (
                                    <TableCell align="center" key={application.slug}>
                                        <Typography variant="subtitle2">{application.name}</Typography>
                                        {application.domain ? (
                                            <Typography color="text.secondary" variant="caption">
                                                {application.domain}
                                            </Typography>
                                        ) : null}
                                    </TableCell>
                                ))}
                            </TableRow>
                        </TableHead>
                        <TableBody>
                            {users.map((user) => (
                                <TableRow key={user.username}>
                                    <TableCell>
                                        <Stack spacing={0.25}>
                                            <Stack alignItems="center" direction="row" spacing={1}>
                                                <Typography variant="body2">
                                                    {user.display_name || user.username}
                                                </Typography>
                                                {user.status === "disabled" ? (
                                                    <Chip label={translate("Disabled")} size="small" />
                                                ) : null}
                                            </Stack>
                                            <Typography color="text.secondary" variant="caption">
                                                {user.username} · {user.primary_email}
                                            </Typography>
                                        </Stack>
                                    </TableCell>
                                    {visibleApplications.map((application) => {
                                        const state = application.users.find((item) => item.username === user.username);
                                        return (
                                            <TableCell align="center" key={application.slug}>
                                                <Checkbox
                                                    checked={Boolean(state?.granted)}
                                                    disabled={!state || busy !== "" || !mutationLock.unlocked}
                                                    inputProps={{
                                                        "aria-label": `Access ${user.username} to ${application.name}`,
                                                    }}
                                                    onChange={() =>
                                                        state && mutate(application, state).catch(console.error)
                                                    }
                                                />
                                            </TableCell>
                                        );
                                    })}
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                </TableContainer>
            )}
        </Stack>
    );
};

export default PermissionsView;
