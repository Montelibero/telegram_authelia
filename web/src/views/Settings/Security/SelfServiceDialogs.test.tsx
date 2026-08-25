import { fireEvent, render, screen, waitFor } from "@testing-library/react";

import { updateSelfServiceProfile } from "@services/SelfService";
import { EditProfileDialog } from "@views/Settings/Security/SelfServiceDialogs";

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("@contexts/NotificationsContext", () => ({
    useNotifications: () => ({
        createErrorNotification: vi.fn(),
        createSuccessNotification: vi.fn(),
    }),
}));

vi.mock("@services/SelfService", () => ({
    removeSelfServicePassword: vi.fn(),
    setSelfServicePassword: vi.fn(),
    updateSelfServiceProfile: vi.fn(),
}));

it("updates the display name", async () => {
    const onSaved = vi.fn();
    vi.mocked(updateSelfServiceProfile).mockResolvedValue({
        display_name: "Jane Doe",
        password_enabled: true,
        telegram_linked: true,
        username: "jane",
        version: 3,
    });

    render(
        <EditProfileDialog
            open
            profile={{
                display_name: "John Doe",
                password_enabled: true,
                telegram_linked: true,
                username: "jane",
                version: 2,
            }}
            onClose={vi.fn()}
            onSaved={onSaved}
        />,
    );
    fireEvent.change(screen.getByLabelText("Display Name"), { target: { value: "Jane Doe" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
        expect(updateSelfServiceProfile).toHaveBeenCalledWith(2, "Jane Doe");
        expect(onSaved).toHaveBeenCalledWith(expect.objectContaining({ display_name: "Jane Doe" }));
    });
});
