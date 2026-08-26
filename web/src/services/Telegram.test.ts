import { getTelegramLoginURL } from "@services/Telegram";

it("omits the return URL on the bare portal root", () => {
    window.history.replaceState({}, "", "/");

    expect(getTelegramLoginURL()).toBe("/api/telegram/login");
});

it("builds a Telegram login URL with a relative portal return URL", () => {
    expect(getTelegramLoginURL("/?rd=https%3A%2F%2Fapp.example.com")).toBe(
        "/api/telegram/login?rd=%2F%3Frd%3Dhttps%253A%252F%252Fapp.example.com",
    );
});
