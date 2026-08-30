import { render, screen } from "@testing-library/react";

import { getSelfServiceProfile } from "@services/SelfService";
import SecurityView from "@views/Settings/Security/SecurityView";

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("@mui/material", async () => {
    const actual = await vi.importActual("@mui/material");
    return {
        ...actual,
        useTheme: () => ({
            palette: { grey: { 600: "#999" } },
            spacing: (n: number) => `${(n || 1) * 8}px`,
        }),
    };
});

vi.mock("@hooks/Configuration", () => ({
    useConfiguration: () => [{ password_change_disabled: false }, vi.fn(), false, null],
}));

vi.mock("@contexts/NotificationsContext", () => ({
    useNotifications: () => ({
        createErrorNotification: vi.fn(),
        createSuccessNotification: vi.fn(),
    }),
}));

vi.mock("@hooks/UserInfo", () => ({
    useUserInfoGET: () => [
        { display_name: "John Doe", emails: ["john@example.com"], groups: [] },
        vi.fn(),
        false,
        null,
    ],
}));

vi.mock("@services/UserSessionElevation", () => ({
    getUserSessionElevation: vi.fn().mockResolvedValue({ elevated: false }),
}));

vi.mock("@services/SelfService", () => ({
    getSelfServiceProfile: vi.fn(),
    getTelegramPasswordProofURL: () => "/api/self-service/password/telegram",
}));

beforeEach(() => {
    vi.mocked(getSelfServiceProfile).mockResolvedValue({
        display_name: "John Doe",
        password_enabled: true,
        telegram_linked: true,
        username: "john",
        version: 2,
    });
});

vi.mock("@views/Settings/Common/IdentityVerificationDialog", () => ({
    default: () => <div data-testid="identity-dialog" />,
}));

vi.mock("@views/Settings/Common/SecondFactorDialog", () => ({
    default: () => <div data-testid="second-factor-dialog" />,
}));

vi.mock("@views/Settings/Security/ChangePasswordDialog", () => ({
    default: () => <div data-testid="change-password-dialog" />,
}));

vi.mock("@components/TelegramAccountLink", () => ({
    default: ({ enabled }: { enabled: boolean }) => <div data-testid="telegram-account-link">{String(enabled)}</div>,
}));

it("renders user info and password self-service controls", async () => {
    render(<SecurityView />);
    expect(screen.getByText(/John Doe/)).toBeInTheDocument();
    expect(await screen.findByText("Edit Profile")).toBeInTheDocument();
    expect(screen.getByText("Change Password")).toBeInTheDocument();
    expect(screen.getByText("Disable Password Login")).toBeInTheDocument();
});

it("renders dialogs", async () => {
    render(<SecurityView />);
    expect(screen.getByTestId("identity-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("second-factor-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("change-password-dialog")).toBeInTheDocument();
    expect(await screen.findByText("Edit Profile")).toBeInTheDocument();
});

it("renders Telegram account linking when enabled", async () => {
    document.body.dataset.telegramlogin = "true";
    render(<SecurityView />);
    expect(screen.getByTestId("telegram-account-link")).toHaveTextContent("true");
    expect(await screen.findByText("Edit Profile")).toBeInTheDocument();
});
