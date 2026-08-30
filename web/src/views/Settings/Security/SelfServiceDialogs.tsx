import { useState } from "react";

import { Button, Dialog, DialogActions, DialogContent, DialogTitle, TextField } from "@mui/material";
import { useTranslation } from "react-i18next";

import { useNotifications } from "@contexts/NotificationsContext";
import {
    SelfServiceProfile,
    removeSelfServicePassword,
    setSelfServicePassword,
    updateSelfServiceProfile,
} from "@services/SelfService";

interface CommonProps {
    onClose: () => void;
    onSaved: (profile?: SelfServiceProfile) => void;
    open: boolean;
}

export function EditProfileDialog({ onClose, onSaved, open, profile }: CommonProps & { profile?: SelfServiceProfile }) {
    const { t } = useTranslation("settings");
    const { createErrorNotification, createSuccessNotification } = useNotifications();
    const [displayName, setDisplayName] = useState(profile?.display_name || "");
    const [busy, setBusy] = useState(false);

    const save = async () => {
        if (!profile) return;
        setBusy(true);
        try {
            onSaved(await updateSelfServiceProfile(profile.version, displayName));
            createSuccessNotification(t("Profile updated"));
            onClose();
        } catch {
            createErrorNotification(t("There was an issue updating your profile"));
        } finally {
            setBusy(false);
        }
    };
    return (
        <Dialog open={open} onClose={onClose}>
            <DialogTitle>{t("Edit Profile")}</DialogTitle>
            <DialogContent>
                <TextField
                    fullWidth
                    margin="normal"
                    label={t("Display Name")}
                    value={displayName}
                    onChange={(event) => setDisplayName(event.target.value)}
                />
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose}>{t("Cancel")}</Button>
                <Button disabled={busy} onClick={save}>
                    {t("Save")}
                </Button>
            </DialogActions>
        </Dialog>
    );
}

export function SetPasswordDialog({ onClose, onSaved, open }: CommonProps) {
    const { t } = useTranslation("settings");
    const { createErrorNotification, createSuccessNotification } = useNotifications();
    const [password, setPassword] = useState("");
    const [repeat, setRepeat] = useState("");
    const [busy, setBusy] = useState(false);
    const save = async () => {
        if (!password || password !== repeat) return;
        setBusy(true);
        try {
            await setSelfServicePassword(password);
            onSaved();
            createSuccessNotification(t("Password configured successfully"));
            onClose();
        } catch {
            createErrorNotification(t("There was an issue setting the password"));
        } finally {
            setBusy(false);
        }
    };
    return (
        <Dialog open={open} onClose={onClose}>
            <DialogTitle>{t("Set Password")}</DialogTitle>
            <DialogContent>
                <TextField
                    fullWidth
                    margin="normal"
                    label={t("New Password")}
                    type="password"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                />
                <TextField
                    fullWidth
                    margin="normal"
                    label={t("Repeat New Password")}
                    type="password"
                    value={repeat}
                    onChange={(event) => setRepeat(event.target.value)}
                />
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose}>{t("Cancel")}</Button>
                <Button disabled={busy || !password || password !== repeat} onClick={save}>
                    {t("Set Password")}
                </Button>
            </DialogActions>
        </Dialog>
    );
}

export function DisablePasswordDialog({
    onClose,
    onSaved,
    open,
    profile,
}: CommonProps & { profile?: SelfServiceProfile }) {
    const { t } = useTranslation("settings");
    const { createErrorNotification, createSuccessNotification } = useNotifications();
    const [password, setPassword] = useState("");
    const [busy, setBusy] = useState(false);
    const remove = async () => {
        if (!profile || !password) return;
        setBusy(true);
        try {
            await removeSelfServicePassword(profile.version, password);
            onSaved();
            createSuccessNotification(t("Password login disabled"));
            onClose();
        } catch {
            createErrorNotification(t("There was an issue disabling password login"));
        } finally {
            setBusy(false);
        }
    };
    return (
        <Dialog open={open} onClose={onClose}>
            <DialogTitle>{t("Disable Password Login")}</DialogTitle>
            <DialogContent>
                <TextField
                    fullWidth
                    margin="normal"
                    label={t("Current Password")}
                    type="password"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                />
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose}>{t("Cancel")}</Button>
                <Button color="error" disabled={busy || !password} onClick={remove}>
                    {t("Disable")}
                </Button>
            </DialogActions>
        </Dialog>
    );
}
