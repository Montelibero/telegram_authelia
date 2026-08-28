import { fireEvent, render, screen, waitFor } from "@testing-library/react";

import { AssertionResult } from "@models/WebAuthn";
import {
    addAdminGroupUser,
    addAdminUserEmail,
    createAdminUser,
    deleteAdminUserEmail,
    generateAdminUserSetupLink,
    getAdminApplications,
    getAdminGroup,
    getAdminStatus,
    getAdminUser,
    getAdminUsers,
    linkAdminUserTelegram,
    removeAdminGroupUser,
    setAdminUserPrimaryEmail,
    unlinkAdminUserIdentity,
    updateAdminUser,
} from "@services/Admin";
import { postFirstFactorReauthenticate } from "@services/Password";
import { getWebAuthnOptions, getWebAuthnResult, postWebAuthnReauthenticateResponse } from "@services/WebAuthn";
import UsersView from "@views/Settings/Admin/UsersView";

const notifyError = vi.fn();
const notifySuccess = vi.fn();
const writeText = vi.fn();

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }));
vi.mock("@contexts/NotificationsContext", () => ({
    useNotifications: () => ({ createErrorNotification: notifyError, createSuccessNotification: notifySuccess }),
}));
vi.mock("@services/Admin", () => ({
    addAdminGroupUser: vi.fn(),
    addAdminUserEmail: vi.fn(),
    createAdminUser: vi.fn(),
    deleteAdminUserEmail: vi.fn(),
    generateAdminUserSetupLink: vi.fn(),
    getAdminApplications: vi.fn(),
    getAdminGroup: vi.fn(),
    getAdminStatus: vi.fn(),
    getAdminUser: vi.fn(),
    getAdminUsers: vi.fn(),
    linkAdminUserTelegram: vi.fn(),
    removeAdminGroupUser: vi.fn(),
    setAdminUserPrimaryEmail: vi.fn(),
    unlinkAdminUserIdentity: vi.fn(),
    updateAdminUser: vi.fn(),
}));
vi.mock("@services/Password", () => ({ postFirstFactorReauthenticate: vi.fn() }));
vi.mock("@services/WebAuthn", () => ({
    getWebAuthnOptions: vi.fn(),
    getWebAuthnResult: vi.fn(),
    postWebAuthnReauthenticateResponse: vi.fn(),
}));

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
    telegram_id: "42",
};

beforeEach(() => {
    vi.resetAllMocks();
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    vi.mocked(getAdminUsers).mockResolvedValue([summary]);
    vi.mocked(getAdminUser).mockResolvedValue(details);
    vi.mocked(getAdminApplications).mockResolvedValue([
        { domain: "", group: "app:grafana", group_version: 1, name: "app:grafana", slug: "app:grafana", users: [] },
        { domain: "", group: "users", group_version: 1, name: "users", slug: "users", users: [] },
    ]);
    vi.mocked(getAdminStatus).mockResolvedValue({ mutation_ready: true, username: "admin" });
});

it("shows only reauthentication controls for mutations while the administrator proof is stale", async () => {
    vi.mocked(getAdminStatus).mockResolvedValue({ mutation_ready: false, username: "admin" });
    vi.mocked(postFirstFactorReauthenticate).mockResolvedValue(undefined);
    render(<UsersView currentUsername="admin" />);

    expect(await screen.findByRole("button", { name: "Reauthenticate" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create user" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: /alice/i }));
    expect(await screen.findByRole("button", { name: "Save user" })).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Administrator password"), { target: { value: "secret" } });
    fireEvent.click(screen.getByRole("button", { name: "Reauthenticate" }));

    await waitFor(() => expect(postFirstFactorReauthenticate).toHaveBeenCalledWith("secret"));
    expect(screen.getByRole("button", { name: "Create user" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Save user" })).toBeEnabled();
});

it("reauthenticates administrator mutations with a passkey", async () => {
    let reauthenticated = false;
    vi.mocked(getAdminStatus).mockImplementation(async () => ({
        mutation_ready: reauthenticated,
        username: "admin",
    }));
    vi.mocked(getWebAuthnOptions).mockResolvedValue({ options: {} as any, status: 200 });
    vi.mocked(getWebAuthnResult).mockResolvedValue({ response: "assertion" as any, result: AssertionResult.Success });
    vi.mocked(postWebAuthnReauthenticateResponse).mockImplementation(async () => {
        reauthenticated = true;
        return { data: { status: "OK" }, status: 200 } as any;
    });

    render(<UsersView currentUsername="admin" />);

    fireEvent.click(await screen.findByRole("button", { name: "Reauthenticate with a passkey" }));

    await waitFor(() => expect(postWebAuthnReauthenticateResponse).toHaveBeenCalledWith("assertion"));
    await waitFor(() => expect(screen.getByRole("button", { name: "Create user" })).toBeEnabled());
});

it("loads users and opens user details", async () => {
    render(<UsersView currentUsername="admin" />);

    expect(await screen.findByText(/alice@example.com/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /alice/i }));

    expect(await screen.findByDisplayValue("Alice")).toBeInTheDocument();
    expect(getAdminUser).toHaveBeenCalledWith("alice");
});

it("filters users locally by username, display name, and email", async () => {
    vi.mocked(getAdminUsers).mockResolvedValue([
        summary,
        {
            ...summary,
            display_name: "Bob Builder",
            primary_email: "builder@example.com",
            username: "bob",
        },
    ]);
    render(<UsersView currentUsername="admin" />);

    await screen.findByText(/alice@example.com/);
    fireEvent.change(screen.getByLabelText("Filter users"), { target: { value: "BUILDER@" } });

    expect(screen.queryByText(/alice@example.com/)).not.toBeInTheDocument();
    expect(screen.getByText(/builder@example.com/)).toBeInTheDocument();
});

it("creates a user", async () => {
    vi.mocked(createAdminUser).mockResolvedValue(details);
    render(<UsersView currentUsername="admin" />);

    fireEvent.click(await screen.findByRole("button", { name: "Create user" }));
    fireEvent.change(screen.getByLabelText("Username"), { target: { value: "alice" } });
    fireEvent.change(screen.getByLabelText("Display name"), { target: { value: "Alice" } });
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "alice@example.com" } });
    fireEvent.change(screen.getByLabelText("Telegram ID"), { target: { value: "987654321" } });
    fireEvent.click(screen.getByRole("checkbox", { name: "users" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "app:grafana" }));
    fireEvent.click(screen.getByRole("button", { name: "Save new user" }));

    await waitFor(() =>
        expect(createAdminUser).toHaveBeenCalledWith({
            display_name: "Alice",
            email: "alice@example.com",
            groups: ["users", "app:grafana"],
            telegram_id: "987654321",
            username: "alice",
        }),
    );
});

it("creates an email-only user and immediately shows a copyable password setup link", async () => {
    vi.mocked(createAdminUser).mockResolvedValue({
        ...details,
        identities: [],
        password_enabled: false,
        provisioning_status: "awaiting_password_setup",
        telegram_id: undefined,
    });
    vi.mocked(generateAdminUserSetupLink).mockResolvedValue({
        expires_at: "2026-08-25T17:00:00Z",
        setup_url: "https://auth.example/reset-password/step2?token=setup",
    });
    render(<UsersView currentUsername="admin" />);

    fireEvent.click(await screen.findByRole("button", { name: "Create user" }));
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "new@example.com" } });
    fireEvent.click(screen.getByRole("button", { name: "Save new user" }));

    expect(await screen.findByDisplayValue(/token=setup/)).toBeInTheDocument();
    expect(createAdminUser).toHaveBeenCalledWith({
        display_name: "",
        email: "new@example.com",
        groups: [],
        telegram_id: "",
        username: "",
    });
    expect(generateAdminUserSetupLink).toHaveBeenCalledWith("alice");
});

it("creates a Telegram-only user without requiring username or email", async () => {
    vi.mocked(createAdminUser).mockResolvedValue({
        ...details,
        primary_email: "_telegram_987654321@pending.invalid",
        provisioning_status: "awaiting_first_login",
        telegram_id: "987654321",
        username: "_telegram_987654321",
    });
    render(<UsersView currentUsername="admin" />);

    fireEvent.click(await screen.findByRole("button", { name: "Create user" }));
    fireEvent.change(screen.getByLabelText("Telegram ID"), { target: { value: "987654321" } });
    fireEvent.click(screen.getByRole("checkbox", { name: "app:grafana" }));
    fireEvent.click(screen.getByRole("button", { name: "Save new user" }));

    await waitFor(() =>
        expect(createAdminUser).toHaveBeenCalledWith({
            display_name: "",
            email: "",
            groups: ["app:grafana"],
            telegram_id: "987654321",
            username: "",
        }),
    );
    expect(generateAdminUserSetupLink).not.toHaveBeenCalled();
});

it("links a Telegram ID to an existing user", async () => {
    const withoutIdentity = { ...details, identities: [], telegram_id: undefined };
    vi.mocked(getAdminUser).mockResolvedValue(withoutIdentity);
    vi.mocked(linkAdminUserTelegram).mockResolvedValue({
        ...withoutIdentity,
        identities: [{ ...details.identities[0], provider_user_id: "987654321" }],
        version: 2,
    });
    render(<UsersView currentUsername="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: /alice/i }));

    fireEvent.change(await screen.findByLabelText("Telegram ID"), { target: { value: "987654321" } });
    fireEvent.click(screen.getByRole("button", { name: "Link Telegram" }));

    await waitFor(() => expect(linkAdminUserTelegram).toHaveBeenCalledWith("alice", "987654321", 1));
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
    writeText.mockResolvedValue(undefined);
    fireEvent.click(screen.getByRole("button", { name: "Copy link" }));
    await waitFor(() =>
        expect(writeText).toHaveBeenCalledWith("https://auth.example/reset-password/step2?token=secret"),
    );
    expect(notifySuccess).toHaveBeenCalledWith("Setup link copied");
});

it("reports a clipboard failure without exposing the setup link", async () => {
    vi.mocked(generateAdminUserSetupLink).mockResolvedValue({
        expires_at: "2026-08-25T17:00:00Z",
        setup_url: "https://auth.example/reset-password/step2?token=secret",
    });
    writeText.mockRejectedValue(new Error("clipboard denied"));
    render(<UsersView currentUsername="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: /alice/i }));
    fireEvent.click(await screen.findByRole("button", { name: "Generate setup link" }));
    await screen.findByDisplayValue(/token=secret/);

    fireEvent.click(screen.getByRole("button", { name: "Copy link" }));

    await waitFor(() => expect(notifyError).toHaveBeenCalledWith("Failed to copy setup link"));
    expect(notifyError).not.toHaveBeenCalledWith(expect.stringContaining("token=secret"));
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

it("adds and removes user group memberships", async () => {
    vi.mocked(addAdminGroupUser).mockResolvedValue({
        managed: false,
        name: "app:grafana",
        updated_at: "2026-08-25T00:00:00Z",
        user_count: 1,
        users: ["alice"],
        version: 5,
    });
    vi.mocked(removeAdminGroupUser).mockResolvedValue({
        managed: false,
        name: "users",
        updated_at: "2026-08-25T00:00:00Z",
        user_count: 0,
        users: [],
        version: 3,
    });
    vi.mocked(getAdminGroup).mockImplementation(async (name) => ({
        managed: false,
        name,
        updated_at: "2026-08-25T00:00:00Z",
        user_count: name === "users" ? 1 : 0,
        users: name === "users" ? ["alice"] : [],
        version: name === "users" ? 2 : 4,
    }));
    render(<UsersView currentUsername="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: /alice/i }));
    await screen.findByDisplayValue("Alice");

    fireEvent.click(screen.getByRole("checkbox", { name: "app:grafana" }));
    await waitFor(() => expect(addAdminGroupUser).toHaveBeenCalledWith("app:grafana", "alice", 4));

    fireEvent.click(screen.getByRole("checkbox", { name: "users" }));
    await waitFor(() => expect(removeAdminGroupUser).toHaveBeenCalledWith("users", "alice", 2, ""));
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
