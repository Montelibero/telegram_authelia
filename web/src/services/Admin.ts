import axios, { AxiosRequestConfig } from "axios";

import {
    AdminApplicationUserPath,
    AdminApplicationsPath,
    AdminGroupPath,
    AdminGroupUserPath,
    AdminGroupsPath,
    AdminPath,
    AdminRegistrationApprovePath,
    AdminRegistrationPath,
    AdminRegistrationRejectPath,
    AdminRegistrationsPath,
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

export interface AdminApplicationUser {
    username: string;
    display_name: string;
    status: "active" | "disabled";
    version: number;
    primary_email: string;
    granted: boolean;
}

export interface AdminApplication {
    slug: string;
    name: string;
    domain: string;
    group: string;
    group_version: number;
    users: AdminApplicationUser[];
}

export interface AdminUserSummary {
    username: string;
    display_name: string;
    status: "active" | "disabled";
    version: number;
    password_enabled: boolean;
    primary_email: string;
    groups: string[];
    provisioning_status?: "awaiting_first_login" | "awaiting_password_setup";
    telegram_id?: string;
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
    telegram_id?: string;
}

export interface AdminUserSetupLink {
    setup_url: string;
    expires_at: string;
}

export interface AdminStatus {
    username: string;
    password_fresh: boolean;
}

export interface AdminRegistration {
    id: number;
    provider: string;
    provider_user_id: string;
    provider_username?: string;
    display_name?: string;
    proposed_username?: string;
    proposed_email?: string;
    status: "approved" | "pending" | "rejected";
    version: number;
    requested_at: string;
    updated_at: string;
    resolved_at?: string;
}

export interface AdminRegistrationApproval {
    id: number;
    expected_version: number;
    username: string;
    display_name: string;
    email: string;
    groups: string[];
}

export interface AdminGroupSummary {
    name: string;
    version: number;
    user_count: number;
    updated_at: string;
    managed: boolean;
}

export interface AdminGroupDetails extends AdminGroupSummary {
    users: string[];
}

export interface AdminGroupWarning {
    group?: AdminGroupDetails;
    affected_users: string[];
    external_acl_not_updated: boolean;
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

export function getAdminStatus() {
    return adminRequest<AdminStatus>({ method: "GET", url: AdminPath });
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

export function linkAdminUserTelegram(username: string, telegramID: string, expectedVersion: number) {
    return adminRequest<AdminUserDetails>({
        data: {
            expected_version: expectedVersion,
            provider: "telegram",
            provider_user_id: telegramID,
            username,
        },
        method: "PUT",
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

export function getAdminRegistrations(status?: AdminRegistration["status"]) {
    const query = status ? `?${new URLSearchParams({ status }).toString()}` : "";
    return adminRequest<AdminRegistration[]>({ method: "GET", url: `${AdminRegistrationsPath}${query}` });
}

export function getAdminRegistration(id: number) {
    return adminRequest<AdminRegistration>({
        method: "GET",
        url: `${AdminRegistrationPath}?${new URLSearchParams({ id: String(id) }).toString()}`,
    });
}

export function approveAdminRegistration(approval: AdminRegistrationApproval) {
    return adminRequest<{ username: string }>({
        data: approval,
        method: "POST",
        url: AdminRegistrationApprovePath,
    });
}

export function rejectAdminRegistration(id: number, expectedVersion: number) {
    return adminRequest<AdminRegistration>({
        data: { expected_version: expectedVersion, id },
        method: "POST",
        url: AdminRegistrationRejectPath,
    });
}

export function getAdminGroups() {
    return adminRequest<AdminGroupSummary[]>({ method: "GET", url: AdminGroupsPath });
}

export function getAdminGroup(name: string) {
    return adminRequest<AdminGroupDetails>({
        method: "GET",
        url: `${AdminGroupPath}?${new URLSearchParams({ name }).toString()}`,
    });
}

export function createAdminGroup(name: string) {
    return adminRequest<AdminGroupDetails>({ data: { name }, method: "POST", url: AdminGroupsPath });
}

export function renameAdminGroup(name: string, newName: string, expectedVersion: number, confirmUsername = "") {
    return adminRequest<AdminGroupWarning>({
        data: {
            confirm_username: confirmUsername,
            expected_version: expectedVersion,
            name,
            new_name: newName,
        },
        method: "PATCH",
        url: AdminGroupPath,
    });
}

export function deleteAdminGroup(name: string, expectedVersion: number, confirmUsername = "") {
    return adminRequest<AdminGroupWarning>({
        data: { confirm_username: confirmUsername, expected_version: expectedVersion, name },
        method: "DELETE",
        url: AdminGroupPath,
    });
}

export function addAdminGroupUser(name: string, username: string, expectedVersion: number) {
    return adminRequest<AdminGroupDetails>({
        data: { expected_version: expectedVersion, name, username },
        method: "PUT",
        url: AdminGroupUserPath,
    });
}

export function removeAdminGroupUser(name: string, username: string, expectedVersion: number, confirmUsername = "") {
    return adminRequest<AdminGroupDetails>({
        data: { confirm_username: confirmUsername, expected_version: expectedVersion, name, username },
        method: "DELETE",
        url: AdminGroupUserPath,
    });
}

export function getAdminApplications() {
    return adminRequest<AdminApplication[]>({ method: "GET", url: AdminApplicationsPath });
}

export function grantAdminApplicationUser(slug: string, username: string, expectedVersion: number) {
    return adminRequest<AdminApplication[]>({
        data: { expected_version: expectedVersion, slug, username },
        method: "PUT",
        url: AdminApplicationUserPath,
    });
}

export function revokeAdminApplicationUser(slug: string, username: string, expectedVersion: number) {
    return adminRequest<AdminApplication[]>({
        data: { expected_version: expectedVersion, slug, username },
        method: "DELETE",
        url: AdminApplicationUserPath,
    });
}
