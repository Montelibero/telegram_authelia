import { fireEvent, render, screen, waitFor } from "@testing-library/react";

import TelegramAccountLink from "@components/TelegramAccountLink";
import { getTelegramLinkStatus, unlinkTelegram } from "@services/Telegram";

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }));
vi.mock("@services/Telegram", async () => {
    const actual = await vi.importActual("@services/Telegram");
    return { ...actual, getTelegramLinkStatus: vi.fn(), unlinkTelegram: vi.fn() };
});

it("shows the unlinked action", async () => {
    vi.mocked(getTelegramLinkStatus).mockResolvedValue({ linked: false });
    render(<TelegramAccountLink enabled={true} />);

    expect(await screen.findByRole("link", { name: "Connect Telegram" })).toHaveAttribute("href", "/api/telegram/link");
});

it("delegates connection so the settings view can elevate first", async () => {
    const onConnect = vi.fn();
    vi.mocked(getTelegramLinkStatus).mockResolvedValue({ linked: false });
    render(<TelegramAccountLink enabled={true} onConnect={onConnect} />);

    fireEvent.click(await screen.findByRole("button", { name: "Connect Telegram" }));
    expect(onConnect).toHaveBeenCalledOnce();
});

it("shows and unlinks a linked Telegram account", async () => {
    vi.mocked(getTelegramLinkStatus).mockResolvedValue({ linked: true, provider_username: "bublik_tg" });
    vi.mocked(unlinkTelegram).mockResolvedValue();
    render(<TelegramAccountLink enabled={true} />);

    expect(await screen.findByText("@bublik_tg")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Disconnect Telegram" }));
    await waitFor(() => expect(unlinkTelegram).toHaveBeenCalledOnce());
    expect(await screen.findByRole("link", { name: "Connect Telegram" })).toBeInTheDocument();
});

it("shows a generic error when status loading fails", async () => {
    vi.mocked(getTelegramLinkStatus).mockRejectedValue(new Error("failed"));
    render(<TelegramAccountLink enabled={true} />);

    expect(await screen.findByText("Unable to load Telegram account status")).toBeInTheDocument();
});

it("does not render when Telegram is disabled", () => {
    render(<TelegramAccountLink enabled={false} />);
    expect(screen.queryByText("Telegram")).not.toBeInTheDocument();
});
