import { act, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

import { useAutheliaState } from "@hooks/State";
import SettingsRouter from "@views/Settings/SettingsRouter";

const mockNavigate = vi.fn();

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("@hooks/State", () => ({
    useAutheliaState: vi.fn(),
}));

vi.mock("@hooks/RouterNavigate", () => ({
    useRouterNavigate: () => mockNavigate,
}));

vi.mock("@constants/Routes", () => ({
    AdminGroupsSubRoute: "/admin/groups",
    AdminPendingSubRoute: "/admin/pending",
    AdminUsersSubRoute: "/admin/users",
    IndexRoute: "/",
    SecuritySubRoute: "/security",
    SettingsRoute: "/settings",
    SettingsTwoFactorAuthenticationSubRoute: "/two-factor-authentication",
}));

vi.mock("@layouts/SettingsLayout", () => ({
    default: (props: any) => <div data-administrator={props.administrator}>{props.children}</div>,
}));

vi.mock("@views/Settings/Admin/UsersView", () => ({
    default: () => <div data-testid="admin-users-view" />,
}));

vi.mock("@views/Settings/Admin/PendingView", () => ({
    default: () => <div data-testid="admin-pending-view" />,
}));

vi.mock("@views/Settings/Admin/GroupsView", () => ({
    default: () => <div data-testid="admin-groups-view" />,
}));

vi.mock("@views/Settings/SettingsView", () => ({
    default: () => <div data-testid="settings-view" />,
}));

vi.mock("@views/Settings/Security/SecurityView", () => ({
    default: () => <div data-testid="security-view" />,
}));

vi.mock("@views/Settings/TwoFactorAuthentication/TwoFactorAuthenticationView", () => ({
    default: () => <div data-testid="2fa-view" />,
}));

beforeEach(() => {
    mockNavigate.mockReset();
});

afterEach(() => {
    vi.restoreAllMocks();
});

it("renders without crashing", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.mocked(useAutheliaState).mockReturnValue([
        { authentication_level: 1, factor_knowledge: true, username: "test" },
        vi.fn(),
        false,
        undefined,
    ]);
    await act(async () => {
        render(
            <MemoryRouter initialEntries={["/settings"]}>
                <SettingsRouter />
            </MemoryRouter>,
        );
    });
});

it("unauthenticated state calls navigate to index route", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.mocked(useAutheliaState).mockReturnValue([
        { authentication_level: 0, factor_knowledge: false, username: "" },
        vi.fn(),
        false,
        undefined,
    ]);
    await act(async () => {
        render(
            <MemoryRouter initialEntries={["/settings"]}>
                <SettingsRouter />
            </MemoryRouter>,
        );
    });
    expect(mockNavigate).toHaveBeenCalledWith("/");
});

it("fetchStateError calls navigate to index route", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.mocked(useAutheliaState).mockReturnValue([undefined, vi.fn(), false, new Error("test")]);
    await act(async () => {
        render(
            <MemoryRouter initialEntries={["/settings"]}>
                <SettingsRouter />
            </MemoryRouter>,
        );
    });
    expect(mockNavigate).toHaveBeenCalledWith("/");
});

it("authenticated state does not call navigate", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.mocked(useAutheliaState).mockReturnValue([
        { authentication_level: 1, factor_knowledge: true, username: "test" },
        vi.fn(),
        false,
        undefined,
    ]);
    await act(async () => {
        render(
            <MemoryRouter initialEntries={["/settings"]}>
                <SettingsRouter />
            </MemoryRouter>,
        );
    });
    expect(mockNavigate).not.toHaveBeenCalled();
});

it("renders admin users route for an administrator", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.mocked(useAutheliaState).mockReturnValue([
        { administrator: true, authentication_level: 1, factor_knowledge: true, username: "admin" },
        vi.fn(),
        false,
        undefined,
    ]);
    await act(async () => {
        render(
            <MemoryRouter initialEntries={["/admin/users"]}>
                <SettingsRouter />
            </MemoryRouter>,
        );
    });
    expect(screen.getByTestId("admin-users-view")).toBeInTheDocument();
});

it("redirects a non-administrator away from an admin route", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.mocked(useAutheliaState).mockReturnValue([
        { authentication_level: 1, factor_knowledge: true, username: "user" },
        vi.fn(),
        false,
        undefined,
    ]);
    await act(async () => {
        render(
            <MemoryRouter initialEntries={["/admin/users"]}>
                <SettingsRouter />
            </MemoryRouter>,
        );
    });
    expect(mockNavigate).toHaveBeenCalledWith("/settings");
});
