import { fireEvent, render, screen, waitFor } from "@testing-library/react";

import {
    addAdminGroupUser,
    createAdminGroup,
    deleteAdminGroup,
    getAdminGroup,
    getAdminGroups,
    getAdminStatus,
    removeAdminGroupUser,
    renameAdminGroup,
} from "@services/Admin";
import { postFirstFactorReauthenticate } from "@services/Password";
import GroupsView from "@views/Settings/Admin/GroupsView";

const notifyError = vi.fn();
const notifySuccess = vi.fn();

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }));
vi.mock("@contexts/NotificationsContext", () => ({
    useNotifications: () => ({ createErrorNotification: notifyError, createSuccessNotification: notifySuccess }),
}));
vi.mock("@services/Admin", () => ({
    addAdminGroupUser: vi.fn(),
    createAdminGroup: vi.fn(),
    deleteAdminGroup: vi.fn(),
    getAdminGroup: vi.fn(),
    getAdminGroups: vi.fn(),
    getAdminStatus: vi.fn(),
    removeAdminGroupUser: vi.fn(),
    renameAdminGroup: vi.fn(),
}));
vi.mock("@services/Password", () => ({ postFirstFactorReauthenticate: vi.fn() }));

const summary = { managed: false, name: "team, odd", updated_at: "2026-08-25T00:00:00Z", user_count: 1, version: 2 };
const details = { ...summary, users: ["alice"] };

beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(getAdminGroups).mockResolvedValue([summary]);
    vi.mocked(getAdminGroup).mockResolvedValue(details);
    vi.mocked(getAdminStatus).mockResolvedValue({ mutation_ready: true, username: "admin" });
});

it("loads groups and group details", async () => {
    render(<GroupsView currentUsername="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: /team, odd/ }));
    expect(await screen.findByDisplayValue("team, odd")).toBeInTheDocument();
    expect(screen.getByText("alice")).toBeInTheDocument();
});

it("reauthenticates before group mutations", async () => {
    vi.mocked(getAdminStatus).mockResolvedValue({ mutation_ready: false, username: "admin" });
    vi.mocked(postFirstFactorReauthenticate).mockResolvedValue(undefined);
    render(<GroupsView currentUsername="admin" />);
    fireEvent.change(await screen.findByLabelText("Administrator password"), { target: { value: "secret" } });
    fireEvent.click(screen.getByRole("button", { name: "Reauthenticate" }));
    await waitFor(() => expect(postFirstFactorReauthenticate).toHaveBeenCalledWith("secret"));
});

it("creates an unrestricted group name", async () => {
    vi.mocked(createAdminGroup).mockResolvedValue(details);
    render(<GroupsView currentUsername="admin" />);
    fireEvent.change(screen.getByLabelText("New group name"), { target: { value: "team, odd" } });
    const createButton = screen.getByRole("button", { name: "Create group" });
    await waitFor(() => expect(createButton).toBeEnabled());
    fireEvent.click(createButton);
    await waitFor(() => expect(createAdminGroup).toHaveBeenCalledWith("team, odd"));
});

it("renames and deletes a group while surfacing the external ACL warning", async () => {
    vi.mocked(renameAdminGroup).mockResolvedValue({
        affected_users: ["alice"],
        external_acl_not_updated: true,
        group: { ...details, name: "renamed", version: 3 },
    });
    vi.mocked(deleteAdminGroup).mockResolvedValue({
        affected_users: ["alice"],
        external_acl_not_updated: true,
        group: { managed: false, name: "", updated_at: "", user_count: 0, users: [], version: 0 },
    });
    render(<GroupsView currentUsername="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: /team, odd/ }));
    await screen.findByDisplayValue("team, odd");
    fireEvent.change(screen.getByLabelText("Group name"), { target: { value: "renamed" } });
    fireEvent.click(screen.getByRole("button", { name: "Rename group" }));
    expect(await screen.findByText(/External ACL configuration was not changed/)).toBeInTheDocument();
    expect(screen.getByText(/Affected users: alice/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Delete group" }));
    await waitFor(() => expect(deleteAdminGroup).toHaveBeenCalledWith("renamed", 3, ""));
    await waitFor(() => expect(screen.queryByLabelText("Group name")).not.toBeInTheDocument());
});

it("adds and removes group members", async () => {
    vi.mocked(addAdminGroupUser).mockResolvedValue({ ...details, users: ["alice", "bob"], version: 3 });
    vi.mocked(removeAdminGroupUser).mockResolvedValue({ ...details, users: ["bob"], version: 4 });
    render(<GroupsView currentUsername="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: /team, odd/ }));
    await screen.findByDisplayValue("team, odd");
    fireEvent.change(screen.getByLabelText("Username to add"), { target: { value: "bob" } });
    fireEvent.click(screen.getByRole("button", { name: "Add member" }));
    await waitFor(() => expect(addAdminGroupUser).toHaveBeenCalledWith("team, odd", "bob", 2));
    fireEvent.click(screen.getByRole("button", { name: "Remove alice" }));
    await waitFor(() => expect(removeAdminGroupUser).toHaveBeenCalledWith("team, odd", "alice", 3, ""));
});

it("requires exact typed confirmation for changes affecting the current administrator", async () => {
    vi.mocked(getAdminGroup).mockResolvedValue({ ...details, users: ["admin"] });
    render(<GroupsView currentUsername="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: /team, odd/ }));
    await screen.findByDisplayValue("team, odd");
    expect(screen.getByRole("button", { name: "Rename group" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Delete group" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Remove admin" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Confirmation username"), { target: { value: "admin" } });
    expect(screen.getByRole("button", { name: "Rename group" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Delete group" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Remove admin" })).toBeEnabled();
});

it("keeps application-managed group structure read-only while allowing membership changes", async () => {
    const managed = { ...details, managed: true };
    vi.mocked(getAdminGroups).mockResolvedValue([managed]);
    vi.mocked(getAdminGroup).mockResolvedValue(managed);

    render(<GroupsView currentUsername="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: /team, odd/ }));

    expect(await screen.findByText("Managed application group")).toBeInTheDocument();
    expect(screen.getByLabelText("Group name")).toBeDisabled();
    expect(screen.getByRole("button", { name: "Rename group" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Delete group" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Username to add"), { target: { value: "bob" } });
    expect(screen.getByRole("button", { name: "Add member" })).toBeEnabled();
});

it("refreshes group details after a version conflict", async () => {
    vi.mocked(renameAdminGroup).mockRejectedValue({ response: { status: 409 } });
    vi.mocked(getAdminGroup)
        .mockResolvedValueOnce(details)
        .mockResolvedValueOnce({ ...details, name: "changed elsewhere", version: 3 });
    render(<GroupsView currentUsername="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: /team, odd/ }));
    await screen.findByDisplayValue("team, odd");
    fireEvent.click(screen.getByRole("button", { name: "Rename group" }));
    expect(await screen.findByDisplayValue("changed elsewhere")).toBeInTheDocument();
});
