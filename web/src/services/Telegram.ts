import { TelegramLoginPath } from "@services/Api";

export function getTelegramLoginURL(returnURL = `${window.location.pathname}${window.location.search}`) {
    const query = new URLSearchParams({ rd: returnURL });
    return `${TelegramLoginPath}?${query.toString()}`;
}
