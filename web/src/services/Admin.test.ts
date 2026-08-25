import axios from "axios";

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

vi.mock("axios");

const mockedAxios = vi.mocked(axios);

beforeEach(() => mockedAxios.mockReset());

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
