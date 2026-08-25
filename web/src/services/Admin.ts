import axios, { AxiosRequestConfig } from "axios";

import {
    AdminUserEmailPath,
    AdminUserIdentityPath,
    AdminUserPath,
    AdminUserPrimaryEmailPath,
    AdminUserSetupLinkPath,
    AdminUsersPath,
    ServiceResponse,
    hasServiceError,
    toData,
} from "@services/Api";

export interface AdminUserSummary {
    username: string;
    display_name: string;
    status: "active" | "disabled";
    version: number;
    password_enabled: boolean;
    primary_email: string;
    groups: string[];
}

export interface AdminUserEmail {
    id: number;
    user_id: number;
    email: string;
    primary: boolean;
    verified: boolean;
    created_at: string;
    updated_at: string;
}

export interface AdminUserIdentity {
    id: number;
    user_id: number;
    provider: string;
    provider_user_id: string;
    provider_username?: string;
    created_at: string;
    updated_at: string;
}

export interface AdminUserDetails extends AdminUserSummary {
    session_epoch: number;
    emails: AdminUserEmail[];
    identities: AdminUserIdentity[];
}

export interface AdminUserCreate {
    username: string;
    display_name: string;
    email: string;
    groups: string[];
}

export interface AdminUserSetupLink {
    setup_url: string;
    expires_at: string;
}

async function adminRequest<T>(config: AxiosRequestConfig): Promise<T> {
    const response = await axios<ServiceResponse<T>>(config);
    const serviceError = hasServiceError(response);

    if (response.status < 200 || response.status >= 300 || serviceError.errored) {
        throw new Error(`Admin request failed. Code: ${response.status}. Message: ${serviceError.message}`);
    }

    const data = toData(response);
    if (data === undefined) {
        throw new Error("Admin request returned an unexpected response");
    }

    return data;
}

export function getAdminUsers() {
    return adminRequest<AdminUserSummary[]>({ method: "GET", url: AdminUsersPath });
}

export function getAdminUser(username: string) {
    const query = new URLSearchParams({ username });
    return adminRequest<AdminUserDetails>({ method: "GET", url: `${AdminUserPath}?${query.toString()}` });
}

export function createAdminUser(user: AdminUserCreate) {
    return adminRequest<AdminUserDetails>({ data: user, method: "POST", url: AdminUsersPath });
}

export function updateAdminUser(
    username: string,
    expectedVersion: number,
    displayName: string,
    status: AdminUserSummary["status"],
    confirmUsername = "",
) {
    return adminRequest<AdminUserDetails>({
        data: {
            confirm_username: confirmUsername,
            display_name: displayName,
            expected_version: expectedVersion,
            status,
            username,
        },
        method: "PATCH",
        url: AdminUserPath,
    });
}

export function addAdminUserEmail(username: string, email: string, expectedVersion: number, primary: boolean) {
    return adminRequest<AdminUserDetails>({
        data: { email, expected_version: expectedVersion, primary, username },
        method: "POST",
        url: AdminUserEmailPath,
    });
}

export function setAdminUserPrimaryEmail(username: string, email: string, expectedVersion: number) {
    return adminRequest<AdminUserDetails>({
        data: { email, expected_version: expectedVersion, username },
        method: "PUT",
        url: AdminUserPrimaryEmailPath,
    });
}

export function deleteAdminUserEmail(username: string, email: string, expectedVersion: number) {
    return adminRequest<AdminUserDetails>({
        data: { email, expected_version: expectedVersion, username },
        method: "DELETE",
        url: AdminUserEmailPath,
    });
}

export function unlinkAdminUserIdentity(
    username: string,
    provider: string,
    expectedVersion: number,
    confirmUsername = "",
) {
    return adminRequest<AdminUserDetails>({
        data: { confirm_username: confirmUsername, expected_version: expectedVersion, provider, username },
        method: "DELETE",
        url: AdminUserIdentityPath,
    });
}

export function generateAdminUserSetupLink(username: string) {
    return adminRequest<AdminUserSetupLink>({
        data: { username },
        method: "POST",
        url: AdminUserSetupLinkPath,
    });
}
