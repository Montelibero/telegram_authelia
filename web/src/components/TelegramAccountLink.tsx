import { useEffect, useState } from "react";

import TelegramIcon from "@mui/icons-material/Telegram";
import { Box, Button, CircularProgress, Typography } from "@mui/material";
import { useTranslation } from "react-i18next";

import { TelegramLinkStatus, getTelegramLinkStatus, getTelegramLinkURL, unlinkTelegram } from "@services/Telegram";

export interface Props {
    enabled: boolean;
    onConnect?: () => void;
}

const TelegramAccountLink = function ({ enabled, onConnect }: Props) {
    const { t: translate } = useTranslation("settings");
    const [status, setStatus] = useState<TelegramLinkStatus>();
    const [loading, setLoading] = useState(enabled);
    const [error, setError] = useState(false);

    useEffect(() => {
        if (!enabled) return;
        getTelegramLinkStatus()
            .then(setStatus)
            .catch(() => setError(true))
            .finally(() => setLoading(false));
    }, [enabled]);

    if (!enabled) return null;

    const handleUnlink = async () => {
        setLoading(true);
        setError(false);
        try {
            await unlinkTelegram();
            setStatus({ linked: false });
        } catch {
            setError(true);
        } finally {
            setLoading(false);
        }
    };

    return (
        <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, mt: 2, p: 2 }}>
            <Typography variant="h6" sx={{ mb: 1 }}>
                <TelegramIcon sx={{ mr: 1, verticalAlign: "middle" }} />
                Telegram
            </Typography>
            {error ? (
                <Typography color="error">{translate("Unable to load Telegram account status")}</Typography>
            ) : null}
            {loading ? <CircularProgress size={20} /> : null}
            {!loading && !error && status?.linked ? (
                <Box>
                    <Typography sx={{ mb: 1 }}>
                        {status.provider_username ? `@${status.provider_username}` : status.provider_user_id}
                    </Typography>
                    <Button color="error" variant="outlined" onClick={handleUnlink}>
                        {translate("Disconnect Telegram")}
                    </Button>
                </Box>
            ) : null}
            {!loading && !error && status && !status.linked ? (
                onConnect ? (
                    <Button onClick={onConnect} variant="outlined">
                        {translate("Connect Telegram")}
                    </Button>
                ) : (
                    <Button component="a" href={getTelegramLinkURL()} variant="outlined">
                        {translate("Connect Telegram")}
                    </Button>
                )
            ) : null}
        </Box>
    );
};

export default TelegramAccountLink;
