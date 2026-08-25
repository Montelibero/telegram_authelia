import { fireEvent, render, screen, waitFor } from "@testing-library/react";

import {
    addAdminUserEmail,
    createAdminUser,
    deleteAdminUserEmail,
    generateAdminUserSetupLink,
    getAdminUser,
    getAdminUsers,
    setAdminUserPrimaryEmail,
    unlinkAdminUserIdentity,
    updateAdminUser,
} from "@services/Admin";
import { postFirstFactorReauthenticate } from "@services/Password";
import UsersView from "@views/Settings/Admin/UsersView";

const notifyError = vi.fn();
const notifySuccess = vi.fn();

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }));
vi.mock("@contexts/NotificationsContext", () => ({
    useNotifications: () => ({ createErrorNotification: notifyError, createSuccessNotification: notifySuccess }),
}));
vi.mock("@services/Admin", () => ({
    addAdminUserEmail: vi.fn(),
    createAdminUser: vi.fn(),
    deleteAdminUserEmail: vi.fn(),
    generateAdminUserSetupLink: vi.fn(),
    getAdminUser: vi.fn(),
    getAdminUsers: vi.fn(),
    setAdminUserPrimaryEmail: vi.fn(),
    unlinkAdminUserIdentity: vi.fn(),
    updateAdminUser: vi.fn(),
}));
vi.mock("@services/Password", () => ({ postFirstFactorReauthenticate: vi.fn() }));

const summary = {
    display_name: "Alice",
    groups: ["users"],
    password_enabled: false,
    primary_email: "alice@example.com",
    status: "active" as const,
    username: "alice",
    version: 1,
};

const details = {
    ...summary,
    emails: [
        {
            created_at: "2026-08-25T00:00:00Z",
            email: "alice@example.com",
            id: 1,
            primary: true,
            updated_at: "2026-08-25T00:00:00Z",
            user_id: 1,
            verified: true,
        },
        {
            created_at: "2026-08-25T00:00:00Z",
            email: "secondary@example.com",
            id: 2,
            primary: false,
            updated_at: "2026-08-25T00:00:00Z",
            user_id: 1,
            verified: true,
        },
    ],
    identities: [
        {
            created_at: "2026-08-25T00:00:00Z",
            id: 1,
            provider: "telegram",
            provider_user_id: "42",
            provider_username: "alice",
            updated_at: "2026-08-25T00:00:00Z",
            user_id: 1,
        },
    ],
    session_epoch: 0,
};

beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(getAdminUsers).mockResolvedValue([summary]);
    vi.mocked(getAdminUser).mockResolvedValue(details);
});

it("loads users and opens user details", async () => {
    render(<UsersView currentUsername="admin" />);

    expect(await screen.findByText(/alice@example.com/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /alice/i }));

    expect(await screen.findByDisplayValue("Alice")).toBeInTheDocument();
    expect(getAdminUser).toHaveBeenCalledWith("alice");
});

it("reauthenticates with a password without replacing the current login", async () => {
    vi.mocked(postFirstFactorReauthenticate).mockResolvedValue(undefined);
    render(<UsersView currentUsername="admin" />);

    fireEvent.change(screen.getByLabelText("Administrator password"), { target: { value: "secret" } });
    fireEvent.click(screen.getByRole("button", { name: "Unlock changes" }));

    await waitFor(() => expect(postFirstFactorReauthenticate).toHaveBeenCalledWith("secret"));
    expect(notifySuccess).toHaveBeenCalledWith("Administrator actions unlocked");
});

it("creates a user", async () => {
    vi.mocked(createAdminUser).mockResolvedValue(details);
    render(<UsersView currentUsername="admin" />);

    fireEvent.click(await screen.findByRole("button", { name: "Create user" }));
    fireEvent.change(screen.getByLabelText("Username"), { target: { value: "alice" } });
    fireEvent.change(screen.getByLabelText("Display name"), { target: { value: "Alice" } });
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "alice@example.com" } });
    fireEvent.change(screen.getByLabelText("New group"), { target: { value: "users" } });
    fireEvent.click(screen.getByRole("button", { name: "Add group" }));
    fireEvent.change(screen.getByLabelText("New group"), { target: { value: "app:grafana" } });
    fireEvent.click(screen.getByRole("button", { name: "Add group" }));
    fireEvent.click(screen.getByRole("button", { name: "Save new user" }));

    await waitFor(() =>
        expect(createAdminUser).toHaveBeenCalledWith({
            display_name: "Alice",
            email: "alice@example.com",
            groups: ["users", "app:grafana"],
            username: "alice",
        }),
    );
});

it("updates a user and generates a copyable setup link", async () => {
    vi.mocked(updateAdminUser).mockResolvedValue({ ...details, display_name: "Alice B", version: 2 });
    vi.mocked(generateAdminUserSetupLink).mockResolvedValue({
        expires_at: "2026-08-25T17:00:00Z",
        setup_url: "https://auth.example/reset-password/step2?token=secret",
    });
    render(<UsersView currentUsername="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: /alice/i }));
    await screen.findByDisplayValue("Alice");

    fireEvent.change(screen.getByLabelText("Display name"), { target: { value: "Alice B" } });
    fireEvent.click(screen.getByRole("button", { name: "Save user" }));
    await waitFor(() => expect(updateAdminUser).toHaveBeenCalledWith("alice", 1, "Alice B", "active", ""));

    fireEvent.click(screen.getByRole("button", { name: "Generate setup link" }));
    expect(await screen.findByDisplayValue(/token=secret/)).toBeInTheDocument();
    expect(screen.getByText(/2026/)).toBeInTheDocument();
});

it("manages emails and unlinks identities", async () => {
    vi.mocked(addAdminUserEmail).mockResolvedValue({ ...details, version: 2 });
    vi.mocked(setAdminUserPrimaryEmail).mockResolvedValue({ ...details, version: 2 });
    vi.mocked(deleteAdminUserEmail).mockResolvedValue({ ...details, version: 2 });
    vi.mocked(unlinkAdminUserIdentity).mockResolvedValue({ ...details, identities: [], version: 2 });
    render(<UsersView currentUsername="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: /alice/i }));
    await screen.findByDisplayValue("Alice");

    fireEvent.change(screen.getByLabelText("New email"), { target: { value: "next@example.com" } });
    fireEvent.click(screen.getByRole("button", { name: "Add email" }));
    await waitFor(() => expect(addAdminUserEmail).toHaveBeenCalledWith("alice", "next@example.com", 1, false));

    fireEvent.click(screen.getByRole("button", { name: "Make primary secondary@example.com" }));
    await waitFor(() => expect(setAdminUserPrimaryEmail).toHaveBeenCalled());
    fireEvent.click(screen.getByRole("button", { name: "Delete alice@example.com" }));
    await waitFor(() => expect(deleteAdminUserEmail).toHaveBeenCalled());
    fireEvent.click(screen.getByRole("button", { name: "Unlink telegram" }));
    await waitFor(() => expect(unlinkAdminUserIdentity).toHaveBeenCalledWith("alice", "telegram", 2, ""));
});

it("refreshes user details after an optimistic version conflict", async () => {
    vi.mocked(updateAdminUser).mockRejectedValue({ response: { status: 409 } });
    vi.mocked(getAdminUser)
        .mockResolvedValueOnce(details)
        .mockResolvedValueOnce({ ...details, display_name: "Changed elsewhere", version: 2 });
    render(<UsersView currentUsername="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: /alice/i }));
    await screen.findByDisplayValue("Alice");

    fireEvent.click(screen.getByRole("button", { name: "Save user" }));

    expect(await screen.findByDisplayValue("Changed elsewhere")).toBeInTheDocument();
    expect(getAdminUser).toHaveBeenCalledTimes(2);
});

it("requires typed username before self-disable and last-login unlink", async () => {
    render(<UsersView currentUsername="alice" />);
    fireEvent.click(await screen.findByRole("button", { name: /alice/i }));
    await screen.findByDisplayValue("Alice");

    fireEvent.mouseDown(screen.getByLabelText("Status"));
    fireEvent.click(await screen.findByRole("option", { name: "Disabled" }));
    expect(screen.getByRole("button", { name: "Save user" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Unlink telegram" })).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Confirmation username"), { target: { value: "alice" } });
    expect(screen.getByRole("button", { name: "Save user" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Unlink telegram" })).toBeEnabled();
});
