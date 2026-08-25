import axios from "axios";

import { SelfServicePasswordPath, SelfServicePasswordTelegramPath, SelfServiceProfilePath } from "@services/Api";

export interface SelfServiceProfile {
    display_name: string;
    password_enabled: boolean;
    telegram_linked: boolean;
    username: string;
    version: number;
}

export async function getSelfServiceProfile(): Promise<SelfServiceProfile> {
    const response = await axios.get(SelfServiceProfilePath);
    return response.data.data;
}

export async function updateSelfServiceProfile(expectedVersion: number, displayName: string) {
    const response = await axios.patch(SelfServiceProfilePath, {
        display_name: displayName,
        expected_version: expectedVersion,
    });
    return response.data.data as SelfServiceProfile;
}

export function getTelegramPasswordProofURL() {
    return SelfServicePasswordTelegramPath;
}

export async function setSelfServicePassword(newPassword: string) {
    const response = await axios.post(SelfServicePasswordPath, { new_password: newPassword });
    if (response.status !== 204) throw new Error(`Failed to set password. Code: ${response.status}.`);
}

export async function removeSelfServicePassword(expectedVersion: number, currentPassword: string) {
    const response = await axios.delete(SelfServicePasswordPath, {
        data: { current_password: currentPassword, expected_version: expectedVersion },
    });
    if (response.status !== 204) throw new Error(`Failed to remove password. Code: ${response.status}.`);
}
