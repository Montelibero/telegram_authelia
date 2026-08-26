import axios from "axios";

import {
    addAdminGroupUser,
    addAdminUserEmail,
    approveAdminRegistration,
    createAdminGroup,
    createAdminUser,
    deleteAdminGroup,
    deleteAdminUserEmail,
    generateAdminUserSetupLink,
    getAdminApplications,
    getAdminGroup,
    getAdminGroups,
    getAdminRegistration,
    getAdminRegistrations,
    getAdminStatus,
    getAdminUser,
    getAdminUsers,
    grantAdminApplicationUser,
    linkAdminUserTelegram,
    rejectAdminRegistration,
    removeAdminGroupUser,
    renameAdminGroup,
    revokeAdminApplicationUser,
    setAdminUserPrimaryEmail,
    unlinkAdminUserIdentity,
    updateAdminUser,
} from "@services/Admin";

vi.mock("axios");

const mockedAxios = vi.mocked(axios);

beforeEach(() => mockedAxios.mockReset());

it("loads the current administrator mutation capability", async () => {
    mockedAxios.mockResolvedValueOnce({
        data: { data: { mutation_ready: false, username: "admin" }, status: "OK" },
        status: 200,
    });

    await expect(getAdminStatus()).resolves.toEqual({ mutation_ready: false, username: "admin" });
    expect(mockedAxios).toHaveBeenCalledWith({ method: "GET", url: "/api/admin" });
});

it("loads users and user details", async () => {
    mockedAxios
        .mockResolvedValueOnce({ data: { data: [], status: "OK" }, status: 200 })
        .mockResolvedValueOnce({ data: { data: { username: "alice" }, status: "OK" }, status: 200 });

    await expect(getAdminUsers()).resolves.toEqual([]);
    await expect(getAdminUser("alice & bob")).resolves.toEqual({ username: "alice" });
    expect(mockedAxios).toHaveBeenNthCalledWith(1, { method: "GET", url: "/api/admin/users" });
    expect(mockedAxios).toHaveBeenNthCalledWith(2, {
        method: "GET",
        url: "/api/admin/user?username=alice+%26+bob",
    });
});

it("links a Telegram ID to an existing user", async () => {
    mockedAxios.mockResolvedValue({ data: { data: {}, status: "OK" }, status: 200 } as any);

    await linkAdminUserTelegram("alice", "987654321", 3);

    expect(mockedAxios).toHaveBeenCalledWith({
        data: { expected_version: 3, provider: "telegram", provider_user_id: "987654321", username: "alice" },
        method: "PUT",
        url: "/api/admin/users/identity",
    });
});

it("loads and mutates application permissions", async () => {
    const applications = [
        {
            domain: "grafana.example.com",
            group: "app:grafana",
            group_version: 3,
            name: "Grafana",
            slug: "grafana",
            users: [],
        },
    ];
    mockedAxios.mockResolvedValue({ data: { data: applications, status: "OK" }, status: 200 });

    await expect(getAdminApplications()).resolves.toEqual(applications);
    await expect(grantAdminApplicationUser("grafana", "alice", 3)).resolves.toEqual(applications);
    await expect(revokeAdminApplicationUser("grafana", "alice", 4)).resolves.toEqual(applications);

    expect(mockedAxios).toHaveBeenNthCalledWith(1, { method: "GET", url: "/api/admin/applications" });
    expect(mockedAxios).toHaveBeenNthCalledWith(2, {
        data: { expected_version: 3, slug: "grafana", username: "alice" },
        method: "PUT",
        url: "/api/admin/application/user",
    });
    expect(mockedAxios).toHaveBeenNthCalledWith(3, {
        data: { expected_version: 4, slug: "grafana", username: "alice" },
        method: "DELETE",
        url: "/api/admin/application/user",
    });
});

it("creates and updates a user", async () => {
    mockedAxios.mockResolvedValue({ data: { data: { username: "alice" }, status: "OK" }, status: 200 });

    await createAdminUser({ display_name: "Alice", email: "alice@example.com", groups: ["users"], username: "alice" });
    await updateAdminUser("alice", 2, "Alice B", "disabled", "alice");

    expect(mockedAxios).toHaveBeenNthCalledWith(1, {
        data: { display_name: "Alice", email: "alice@example.com", groups: ["users"], username: "alice" },
        method: "POST",
        url: "/api/admin/users",
    });
    expect(mockedAxios).toHaveBeenNthCalledWith(2, {
        data: {
            confirm_username: "alice",
            display_name: "Alice B",
            expected_version: 2,
            status: "disabled",
            username: "alice",
        },
        method: "PATCH",
        url: "/api/admin/user",
    });
});

it("manages user emails and identities", async () => {
    mockedAxios.mockResolvedValue({ data: { data: { username: "alice" }, status: "OK" }, status: 200 });

    await addAdminUserEmail("alice", "next@example.com", 3, true);
    await setAdminUserPrimaryEmail("alice", "next@example.com", 4);
    await deleteAdminUserEmail("alice", "old@example.com", 5);
    await unlinkAdminUserIdentity("alice", "telegram", 6, "alice");

    expect(mockedAxios).toHaveBeenNthCalledWith(1, {
        data: { email: "next@example.com", expected_version: 3, primary: true, username: "alice" },
        method: "POST",
        url: "/api/admin/users/email",
    });
    expect(mockedAxios).toHaveBeenNthCalledWith(2, {
        data: { email: "next@example.com", expected_version: 4, username: "alice" },
        method: "PUT",
        url: "/api/admin/users/email/primary",
    });
    expect(mockedAxios).toHaveBeenNthCalledWith(3, {
        data: { email: "old@example.com", expected_version: 5, username: "alice" },
        method: "DELETE",
        url: "/api/admin/users/email",
    });
    expect(mockedAxios).toHaveBeenNthCalledWith(4, {
        data: { confirm_username: "alice", expected_version: 6, provider: "telegram", username: "alice" },
        method: "DELETE",
        url: "/api/admin/users/identity",
    });
});

it("generates a one-time setup link", async () => {
    const data = { expires_at: "2026-08-25T17:00:00Z", setup_url: "https://auth.example/reset?token=secret" };
    mockedAxios.mockResolvedValue({ data: { data, status: "OK" }, status: 200 });

    await expect(generateAdminUserSetupLink("alice")).resolves.toEqual(data);
    expect(mockedAxios).toHaveBeenCalledWith({
        data: { username: "alice" },
        method: "POST",
        url: "/api/admin/users/setup-link",
    });
});

it("loads and resolves registrations", async () => {
    mockedAxios.mockResolvedValue({ data: { data: [], status: "OK" }, status: 200 });

    await getAdminRegistrations("pending");
    await getAdminRegistration(42);
    await approveAdminRegistration({
        display_name: "Alice",
        email: "alice@example.com",
        expected_version: 3,
        groups: ["users"],
        id: 42,
        username: "alice",
    });
    await rejectAdminRegistration(42, 3);

    expect(mockedAxios).toHaveBeenNthCalledWith(1, {
        method: "GET",
        url: "/api/admin/registrations?status=pending",
    });
    expect(mockedAxios).toHaveBeenNthCalledWith(2, { method: "GET", url: "/api/admin/registration?id=42" });
    expect(mockedAxios).toHaveBeenNthCalledWith(3, {
        data: {
            display_name: "Alice",
            email: "alice@example.com",
            expected_version: 3,
            groups: ["users"],
            id: 42,
            username: "alice",
        },
        method: "POST",
        url: "/api/admin/registration/approve",
    });
    expect(mockedAxios).toHaveBeenNthCalledWith(4, {
        data: { expected_version: 3, id: 42 },
        method: "POST",
        url: "/api/admin/registration/reject",
    });
});

it("manages groups and memberships", async () => {
    mockedAxios.mockResolvedValue({ data: { data: [], status: "OK" }, status: 200 });

    await getAdminGroups();
    await getAdminGroup("team, odd");
    await createAdminGroup("team, odd");
    await renameAdminGroup("team, odd", "renamed", 2, "admin");
    await deleteAdminGroup("renamed", 3, "admin");
    await addAdminGroupUser("renamed", "alice", 3);
    await removeAdminGroupUser("renamed", "alice", 4, "admin");

    expect(mockedAxios).toHaveBeenNthCalledWith(2, {
        method: "GET",
        url: "/api/admin/group?name=team%2C+odd",
    });
    expect(mockedAxios).toHaveBeenNthCalledWith(3, {
        data: { name: "team, odd" },
        method: "POST",
        url: "/api/admin/groups",
    });
    expect(mockedAxios).toHaveBeenNthCalledWith(4, {
        data: { confirm_username: "admin", expected_version: 2, name: "team, odd", new_name: "renamed" },
        method: "PATCH",
        url: "/api/admin/group",
    });
    expect(mockedAxios).toHaveBeenNthCalledWith(5, {
        data: { confirm_username: "admin", expected_version: 3, name: "renamed" },
        method: "DELETE",
        url: "/api/admin/group",
    });
    expect(mockedAxios).toHaveBeenNthCalledWith(6, {
        data: { expected_version: 3, name: "renamed", username: "alice" },
        method: "PUT",
        url: "/api/admin/group/user",
    });
    expect(mockedAxios).toHaveBeenNthCalledWith(7, {
        data: { confirm_username: "admin", expected_version: 4, name: "renamed", username: "alice" },
        method: "DELETE",
        url: "/api/admin/group/user",
    });
});
