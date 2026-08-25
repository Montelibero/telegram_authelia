import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";

import {
    AdminApplication,
    getAdminApplications,
    grantAdminApplicationUser,
    revokeAdminApplicationUser,
} from "@services/Admin";
import { postFirstFactorReauthenticate } from "@services/Password";
import PermissionsView from "@views/Settings/Admin/PermissionsView";

const notifyError = vi.fn();
const notifySuccess = vi.fn();

const translate = (key: string) => key;
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: translate }) }));
vi.mock("@contexts/NotificationsContext", () => ({
    useNotifications: () => ({ createErrorNotification: notifyError, createSuccessNotification: notifySuccess }),
}));
vi.mock("@services/Admin", () => ({
    getAdminApplications: vi.fn(),
    grantAdminApplicationUser: vi.fn(),
    revokeAdminApplicationUser: vi.fn(),
}));
vi.mock("@services/Password", () => ({ postFirstFactorReauthenticate: vi.fn() }));

const applications: AdminApplication[] = [
    {
        domain: "grafana.example.com",
        group: "app:grafana",
        group_version: 2,
        name: "Grafana",
        slug: "grafana",
        users: [
            {
                display_name: "Alice",
                granted: false,
                primary_email: "alice@example.com",
                status: "active",
                username: "alice",
                version: 1,
            },
            {
                display_name: "Bob",
                granted: true,
                primary_email: "bob@example.com",
                status: "disabled",
                username: "bob",
                version: 1,
            },
        ],
    },
    {
        domain: "wiki.example.com",
        group: "app:wiki",
        group_version: 4,
        name: "Wiki",
        slug: "wiki",
        users: [
            {
                display_name: "Alice",
                granted: true,
                primary_email: "alice@example.com",
                status: "active",
                username: "alice",
                version: 1,
            },
            {
                display_name: "Bob",
                granted: false,
                primary_email: "bob@example.com",
                status: "disabled",
                username: "bob",
                version: 1,
            },
        ],
    },
];

beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(getAdminApplications).mockResolvedValue(applications);
});

it("shows loading state then renders the permission matrix", async () => {
    let resolve!: (value: AdminApplication[]) => void;
    vi.mocked(getAdminApplications).mockReturnValue(new Promise((done) => (resolve = done)));
    render(<PermissionsView />);
    expect(screen.getByRole("progressbar")).toBeInTheDocument();
    await act(async () => resolve(applications));
    expect(await screen.findByRole("checkbox", { name: "Access alice to Grafana" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Access alice to Wiki" })).toBeChecked();
    expect(screen.getByText(/bob@example.com/)).toBeInTheDocument();
});

it("filters users and applications independently", async () => {
    render(<PermissionsView />);
    await screen.findByRole("checkbox", { name: "Access alice to Grafana" });
    fireEvent.change(screen.getByLabelText("Filter users"), { target: { value: "alice@example.com" } });
    expect(screen.queryByRole("checkbox", { name: "Access bob to Grafana" })).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Filter applications"), { target: { value: "wiki" } });
    expect(screen.queryByRole("checkbox", { name: "Access alice to Grafana" })).not.toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Access alice to Wiki" })).toBeInTheDocument();
});

it("reauthenticates with the administrator password", async () => {
    vi.mocked(postFirstFactorReauthenticate).mockResolvedValue(undefined);
    render(<PermissionsView />);
    fireEvent.change(screen.getByLabelText("Administrator password"), { target: { value: "secret" } });
    fireEvent.click(screen.getByRole("button", { name: "Unlock changes" }));
    await waitFor(() => expect(postFirstFactorReauthenticate).toHaveBeenCalledWith("secret"));
    expect(notifySuccess).toHaveBeenCalledWith("Administrator actions unlocked");
});

it("grants and revokes one permission using the current group version", async () => {
    vi.mocked(grantAdminApplicationUser).mockResolvedValue(
        applications.map((application) =>
            application.slug === "grafana"
                ? {
                      ...application,
                      group_version: 3,
                      users: application.users.map((user) =>
                          user.username === "alice" ? { ...user, granted: true } : user,
                      ),
                  }
                : application,
        ),
    );
    vi.mocked(revokeAdminApplicationUser).mockResolvedValue(applications);
    render(<PermissionsView />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "Access alice to Grafana" }));
    await waitFor(() => expect(grantAdminApplicationUser).toHaveBeenCalledWith("grafana", "alice", 2));
    const bob = screen.getByRole("checkbox", { name: "Access bob to Grafana" });
    await waitFor(() => expect(bob).toBeEnabled());
    fireEvent.click(bob);
    await waitFor(() => expect(revokeAdminApplicationUser).toHaveBeenCalledWith("grafana", "bob", 3));
});

it("refreshes the matrix after a version conflict", async () => {
    const refreshed = applications.map((application) => ({
        ...application,
        group_version: application.group_version + 1,
    }));
    vi.mocked(getAdminApplications).mockResolvedValueOnce(applications).mockResolvedValueOnce(refreshed);
    vi.mocked(grantAdminApplicationUser).mockRejectedValue({ response: { status: 409 } });
    render(<PermissionsView />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "Access alice to Grafana" }));
    await waitFor(() => expect(getAdminApplications).toHaveBeenCalledTimes(2));
    expect(notifyError).toHaveBeenCalledWith("Permissions changed elsewhere; the latest version has been loaded");
});
