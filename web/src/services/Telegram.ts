import axios from "axios";

import { TelegramLinkPath, TelegramLinkStatusPath, TelegramLoginPath } from "@services/Api";
import { Get } from "@services/Client";

export interface TelegramLinkStatus {
    linked: boolean;
    provider_user_id?: string;
    provider_username?: string;
}

export function getTelegramLoginURL(returnURL?: string) {
    const target = returnURL ?? `${window.location.pathname}${window.location.search}`;
    if (target === "/") return TelegramLoginPath;

    const query = new URLSearchParams({ rd: target });
    return `${TelegramLoginPath}?${query.toString()}`;
}

export function getTelegramLinkURL() {
    return TelegramLinkPath;
}

export function getTelegramLinkStatus() {
    return Get<TelegramLinkStatus>(TelegramLinkStatusPath);
}

export async function unlinkTelegram() {
    const response = await axios.delete(TelegramLinkPath);
    if (response.status !== 204) throw new Error(`Failed to unlink Telegram. Code: ${response.status}.`);
}
