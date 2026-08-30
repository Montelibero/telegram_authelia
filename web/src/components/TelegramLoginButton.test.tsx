import { render, screen } from "@testing-library/react";

import TelegramLoginButton from "@components/TelegramLoginButton";

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: (key: string) => key }),
}));

it("renders a safe Telegram login link when enabled", () => {
    render(<TelegramLoginButton enabled={true} disabled={false} returnURL="/portal?rd=target" />);

    expect(screen.getByRole("link", { name: "Sign in with Telegram" })).toHaveAttribute(
        "href",
        "/api/telegram/login?rd=%2Fportal%3Frd%3Dtarget",
    );
});

it("does not render when disabled by configuration", () => {
    render(<TelegramLoginButton enabled={false} disabled={false} />);
    expect(screen.queryByRole("link", { name: "Sign in with Telegram" })).not.toBeInTheDocument();
});

it("disables the button while another authentication is loading", () => {
    render(<TelegramLoginButton enabled={true} disabled={true} />);
    expect(screen.getByRole("link", { name: "Sign in with Telegram" })).toHaveAttribute("aria-disabled", "true");
});
