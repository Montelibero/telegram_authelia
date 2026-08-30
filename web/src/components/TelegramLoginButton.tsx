import TelegramIcon from "@mui/icons-material/Telegram";
import { Button } from "@mui/material";
import { useTranslation } from "react-i18next";

import { getTelegramLoginURL } from "@services/Telegram";

export interface Props {
    enabled: boolean;
    disabled: boolean;
    returnURL?: string;
}

const TelegramLoginButton = function ({ disabled, enabled, returnURL }: Props) {
    const { t: translate } = useTranslation();

    if (!enabled) return null;

    return (
        <Button
            id="telegram-login-button"
            component="a"
            href={getTelegramLoginURL(returnURL)}
            variant="outlined"
            fullWidth={true}
            startIcon={<TelegramIcon />}
            disabled={disabled}
        >
            {translate("Sign in with Telegram")}
        </Button>
    );
};

export default TelegramLoginButton;
