import axios from "axios";

import {
    getSelfServiceProfile,
    removeSelfServicePassword,
    setSelfServicePassword,
    updateSelfServiceProfile,
} from "@services/SelfService";

vi.mock("axios");

test("uses the self-service profile and password contracts", async () => {
    vi.mocked(axios.get).mockResolvedValueOnce({ data: { data: { username: "bublik" }, status: "OK" }, status: 200 });
    vi.mocked(axios.patch).mockResolvedValueOnce({
        data: { data: { display_name: "Bublik", username: "bublik" }, status: "OK" },
        status: 200,
    });
    vi.mocked(axios.post).mockResolvedValueOnce({ status: 204 });
    vi.mocked(axios.delete).mockResolvedValueOnce({ status: 204 });

    expect((await getSelfServiceProfile()).username).toBe("bublik");
    expect((await updateSelfServiceProfile(2, "Bublik")).display_name).toBe("Bublik");
    await setSelfServicePassword("new-password");
    await removeSelfServicePassword(3, "current-password");

    expect(axios.patch).toHaveBeenCalledWith(expect.stringContaining("/api/self-service/profile"), {
        display_name: "Bublik",
        expected_version: 2,
    });
    expect(axios.delete).toHaveBeenCalledWith(expect.stringContaining("/api/self-service/password"), {
        data: { current_password: "current-password", expected_version: 3 },
    });
});
