import { useCallback, useEffect, useState } from "react";

import { Alert, Button, Stack, TextField } from "@mui/material";
import { useTranslation } from "react-i18next";

import { useNotifications } from "@contexts/NotificationsContext";
import { getAdminStatus } from "@services/Admin";
import { postFirstFactorReauthenticate } from "@services/Password";

export function isAdminMutationAuthenticationError(error: unknown) {
    const status = (error as { response?: { status?: number } }).response?.status;
    return status === 401 || status === 403;
}

export function useAdminMutationLock() {
    const { t: translate } = useTranslation("settings");
    const { createErrorNotification, createSuccessNotification } = useNotifications();
    const [unlocked, setUnlocked] = useState(false);
    const [password, setPassword] = useState("");
    const [checking, setChecking] = useState(true);

    const refresh = useCallback(async () => {
        try {
            const status = await getAdminStatus();
            setUnlocked(status.mutation_ready);
        } catch {
            setUnlocked(false);
            createErrorNotification(translate("Failed to check administrator authentication"));
        } finally {
            setChecking(false);
        }
    }, [createErrorNotification, translate]);

    useEffect(() => {
        refresh().catch(console.error);
    }, [refresh]);

    const reauthenticate = useCallback(async () => {
        try {
            await postFirstFactorReauthenticate(password);
            setPassword("");
            setUnlocked(true);
            createSuccessNotification(translate("Administrator actions unlocked"));
        } catch {
            setUnlocked(false);
            createErrorNotification(translate("Incorrect password or reauthentication failed"));
        }
    }, [createErrorNotification, createSuccessNotification, password, translate]);

    const lock = useCallback(() => setUnlocked(false), []);

    const controls = unlocked ? null : (
        <Alert severity="warning">
            <Stack direction={{ sm: "row", xs: "column" }} spacing={1} alignItems={{ sm: "center" }}>
                <span>
                    {checking
                        ? translate("Checking administrator authentication")
                        : translate("Reauthenticate to make administrator changes")}
                </span>
                {!checking ? (
                    <>
                        <TextField
                            label={translate("Administrator password")}
                            type="password"
                            value={password}
                            onChange={(event) => setPassword(event.target.value)}
                            size="small"
                        />
                        <Button
                            variant="contained"
                            disabled={!password}
                            onClick={() => reauthenticate().catch(console.error)}
                        >
                            {translate("Reauthenticate")}
                        </Button>
                    </>
                ) : null}
            </Stack>
        </Alert>
    );

    return { controls, lock, unlocked };
}
