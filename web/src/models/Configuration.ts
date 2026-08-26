import { SecondFactorMethod } from "@models/Methods";

export interface Configuration {
    available_methods: Set<SecondFactorMethod>;
    passkey_login_enabled?: boolean;
    password_change_disabled: boolean;
    password_reset_disabled: boolean;
}

export interface SecuritySettingsConfiguration {
    disable: boolean;
}
