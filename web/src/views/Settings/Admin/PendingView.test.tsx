import { fireEvent, render, screen, waitFor } from "@testing-library/react";

import {
    approveAdminRegistration,
    getAdminRegistration,
    getAdminRegistrations,
    rejectAdminRegistration,
} from "@services/Admin";
import { postFirstFactorReauthenticate } from "@services/Password";
import PendingView from "@views/Settings/Admin/PendingView";

const notifyError = vi.fn();
const notifySuccess = vi.fn();

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }));
vi.mock("@contexts/NotificationsContext", () => ({
    useNotifications: () => ({ createErrorNotification: notifyError, createSuccessNotification: notifySuccess }),
}));
vi.mock("@services/Admin", () => ({
    approveAdminRegistration: vi.fn(),
    getAdminRegistration: vi.fn(),
    getAdminRegistrations: vi.fn(),
    rejectAdminRegistration: vi.fn(),
}));
vi.mock("@services/Password", () => ({ postFirstFactorReauthenticate: vi.fn() }));

const registration = {
    display_name: "Telegram Alice",
    id: 42,
    proposed_email: "alice@eurmtl.me",
    proposed_username: "alice",
    provider: "telegram",
    provider_user_id: "123",
    provider_username: "alice",
    requested_at: "2026-08-25T00:00:00Z",
    status: "pending" as const,
    updated_at: "2026-08-25T00:00:00Z",
    version: 3,
};

beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(getAdminRegistrations).mockResolvedValue([registration]);
    vi.mocked(getAdminRegistration).mockResolvedValue(registration);
});

it("loads pending registrations and editable details", async () => {
    render(<PendingView />);
    fireEvent.click(await screen.findByRole("button", { name: /alice/ }));

    expect(await screen.findByDisplayValue("alice@eurmtl.me")).toBeInTheDocument();
    expect(getAdminRegistrations).toHaveBeenCalledWith("pending");
    expect(getAdminRegistration).toHaveBeenCalledWith(42);
});

it("reauthenticates before registration mutations", async () => {
    vi.mocked(postFirstFactorReauthenticate).mockResolvedValue(undefined);
    render(<PendingView />);
    fireEvent.change(screen.getByLabelText("Administrator password"), { target: { value: "secret" } });
    fireEvent.click(screen.getByRole("button", { name: "Unlock changes" }));
    await waitFor(() => expect(postFirstFactorReauthenticate).toHaveBeenCalledWith("secret"));
});

it("edits and approves a pending registration with lossless groups", async () => {
    vi.mocked(approveAdminRegistration).mockResolvedValue({ username: "alice" });
    render(<PendingView />);
    fireEvent.click(await screen.findByRole("button", { name: /alice/ }));
    await screen.findByDisplayValue("alice@eurmtl.me");

    fireEvent.change(screen.getByLabelText("Display name"), { target: { value: "Alice" } });
    fireEvent.change(screen.getByLabelText("New group"), { target: { value: "team, odd" } });
    fireEvent.click(screen.getByRole("button", { name: "Add group" }));
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));

    await waitFor(() =>
        expect(approveAdminRegistration).toHaveBeenCalledWith({
            display_name: "Alice",
            email: "alice@eurmtl.me",
            expected_version: 3,
            groups: ["team, odd"],
            id: 42,
            username: "alice",
        }),
    );
    expect(notifySuccess).toHaveBeenCalled();
});

it("rejects a pending registration", async () => {
    vi.mocked(rejectAdminRegistration).mockResolvedValue({ ...registration, status: "rejected" });
    render(<PendingView />);
    fireEvent.click(await screen.findByRole("button", { name: /alice/ }));
    await screen.findByDisplayValue("alice@eurmtl.me");
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    await waitFor(() => expect(rejectAdminRegistration).toHaveBeenCalledWith(42, 3));
});

it("reloads current registration after a version conflict", async () => {
    vi.mocked(approveAdminRegistration).mockRejectedValue({ response: { status: 409 } });
    vi.mocked(getAdminRegistration)
        .mockResolvedValueOnce(registration)
        .mockResolvedValueOnce({ ...registration, display_name: "Changed elsewhere", version: 4 });
    render(<PendingView />);
    fireEvent.click(await screen.findByRole("button", { name: /alice/ }));
    await screen.findByDisplayValue("alice@eurmtl.me");
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));

    expect(await screen.findByDisplayValue("Changed elsewhere")).toBeInTheDocument();
});
