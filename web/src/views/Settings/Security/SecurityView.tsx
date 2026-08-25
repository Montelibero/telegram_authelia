import { Fragment, useCallback, useEffect, useState } from "react";

import { Box, Button, Container, List, ListItem, Paper, Stack, Tooltip, Typography, useTheme } from "@mui/material";
import { useTranslation } from "react-i18next";

import TelegramAccountLink from "@components/TelegramAccountLink";
import { useNotifications } from "@contexts/NotificationsContext";
import { useConfiguration } from "@hooks/Configuration";
import { useUserInfoGET } from "@hooks/UserInfo";
import { Configuration } from "@models/Configuration";
import { SelfServiceProfile, getSelfServiceProfile, getTelegramPasswordProofURL } from "@services/SelfService";
import { getTelegramLinkURL } from "@services/Telegram";
import { UserSessionElevation, getUserSessionElevation } from "@services/UserSessionElevation";
import { getTelegramLogin } from "@utils/Configuration";
import IdentityVerificationDialog from "@views/Settings/Common/IdentityVerificationDialog";
import SecondFactorDialog from "@views/Settings/Common/SecondFactorDialog";
import ChangePasswordDialog from "@views/Settings/Security/ChangePasswordDialog";
import {
    DisablePasswordDialog,
    EditProfileDialog,
    SetPasswordDialog,
} from "@views/Settings/Security/SelfServiceDialogs";

interface PasswordChangeButtonProps {
    configuration: Configuration | undefined;
    translate: (_key: string) => string;
    handleChangePassword: () => void;
}

const PasswordChangeButton = ({ configuration, handleChangePassword, translate }: PasswordChangeButtonProps) => {
    const buttonContent = (
        <Button
            id="change-password-button"
            variant="contained"
            sx={{ p: 1, width: "100%" }}
            onClick={handleChangePassword}
            disabled={!configuration || configuration.password_change_disabled}
        >
            {translate("Change Password")}
        </Button>
    );

    return !configuration || configuration.password_change_disabled ? (
        <Tooltip title={translate("This is disabled by your administrator")}>
            <Box component={"span"}>{buttonContent}</Box>
        </Tooltip>
    ) : (
        buttonContent
    );
};

const SettingsView = function () {
    const { t: translate } = useTranslation(["settings", "portal"]);
    const theme = useTheme();
    const { createErrorNotification } = useNotifications();

    const [userInfo, fetchUserInfo, , fetchUserInfoError] = useUserInfoGET();
    const [elevation, setElevation] = useState<UserSessionElevation>();
    const [dialogSFOpening, setDialogSFOpening] = useState(false);
    const [dialogIVOpening, setDialogIVOpening] = useState(false);
    const [dialogPWChangeOpen, setDialogPWChangeOpen] = useState(false);
    const [dialogPWChangeOpening, setDialogPWChangeOpening] = useState(false);
    const [dialogTelegramLinkOpening, setDialogTelegramLinkOpening] = useState(false);
    const [configuration, fetchConfiguration, , fetchConfigurationError] = useConfiguration();
    const [profile, setProfile] = useState<SelfServiceProfile>();
    const [profileOpen, setProfileOpen] = useState(false);
    const [setPasswordOpen, setSetPasswordOpen] = useState(
        () => new URLSearchParams(window.location.search).get("telegram_password_setup") === "verified",
    );
    const [disablePasswordOpen, setDisablePasswordOpen] = useState(false);

    const refreshProfile = useCallback(() => {
        getSelfServiceProfile()
            .then(setProfile)
            .catch(() => createErrorNotification(translate("There was an issue retrieving your profile")));
    }, [createErrorNotification, translate]);

    const handleResetStateOpening = () => {
        setDialogSFOpening(false);
        setDialogIVOpening(false);
        setDialogPWChangeOpening(false);
        setDialogTelegramLinkOpening(false);
    };

    const handleResetState = useCallback(() => {
        handleResetStateOpening();

        setElevation(undefined);
        setDialogPWChangeOpen(false);
    }, []);

    const handleOpenChangePWDialog = useCallback(() => {
        handleResetStateOpening();
        setDialogPWChangeOpen(true);
    }, []);

    const handleOpenTelegramLink = useCallback(() => {
        handleResetStateOpening();
        window.location.assign(getTelegramLinkURL());
    }, []);

    const handleSFDialogClosed = (ok: boolean, changed: boolean) => {
        if (!ok) {
            console.warn("Second Factor dialog close callback failed, it was likely cancelled by the user.");

            handleResetState();

            return;
        }

        if (changed) {
            handleElevationRefresh()
                .then((refreshedElevation) => {
                    if (refreshedElevation) {
                        const isElevatedFromRefresh =
                            refreshedElevation.elevated || refreshedElevation.skip_second_factor;
                        if (isElevatedFromRefresh) {
                            setElevation(undefined);
                            if (dialogPWChangeOpening) {
                                handleOpenChangePWDialog();
                            } else if (dialogTelegramLinkOpening) {
                                handleOpenTelegramLink();
                            }
                        } else {
                            setDialogIVOpening(true);
                        }
                    }
                })
                .catch((error) => {
                    console.error(error);
                    createErrorNotification(translate("Failed to get session elevation status"));
                });
        } else {
            const isElevated = elevation && (elevation.elevated || elevation.skip_second_factor);
            if (isElevated) {
                setElevation(undefined);
                if (dialogPWChangeOpening) {
                    handleOpenChangePWDialog();
                } else if (dialogTelegramLinkOpening) {
                    handleOpenTelegramLink();
                }
            } else {
                setDialogIVOpening(true);
            }
        }
    };

    const handleSFDialogOpened = () => {
        setDialogSFOpening(false);
    };

    const handleIVDialogClosed = useCallback(
        (ok: boolean) => {
            if (!ok) {
                console.warn(
                    "Identity Verification dialog close callback failed, it was likely cancelled by the user.",
                );

                handleResetState();

                return;
            }

            setElevation(undefined);
            if (dialogPWChangeOpening) {
                handleOpenChangePWDialog();
            } else if (dialogTelegramLinkOpening) {
                handleOpenTelegramLink();
            }
        },
        [
            dialogPWChangeOpening,
            dialogTelegramLinkOpening,
            handleOpenChangePWDialog,
            handleOpenTelegramLink,
            handleResetState,
        ],
    );

    const handleIVDialogOpened = () => {
        setDialogIVOpening(false);
    };

    const handleElevationRefresh = async () => {
        const result = await getUserSessionElevation();
        setElevation(result);
        return result;
    };

    const handleElevation = () => {
        handleElevationRefresh().catch(console.error);

        setDialogSFOpening(true);
    };

    const handleChangePassword = () => {
        setDialogPWChangeOpening(true);

        handleElevation();
    };

    const handleConnectTelegram = () => {
        setDialogTelegramLinkOpening(true);
        handleElevation();
    };

    useEffect(() => {
        if (fetchUserInfoError) {
            createErrorNotification(translate("There was an issue retrieving user preferences", { ns: "portal" }));
        }
        if (fetchConfigurationError) {
            createErrorNotification(translate("There was an issue retrieving configuration"));
        }
    }, [fetchUserInfoError, fetchConfigurationError, createErrorNotification, translate]);

    useEffect(() => {
        fetchUserInfo();
        fetchConfiguration();
        refreshProfile();
    }, [fetchUserInfo, fetchConfiguration, refreshProfile]);

    return (
        <Fragment>
            <SecondFactorDialog
                info={userInfo}
                elevation={elevation}
                opening={dialogSFOpening}
                handleClosed={handleSFDialogClosed}
                handleOpened={handleSFDialogOpened}
            />
            <IdentityVerificationDialog
                opening={dialogIVOpening}
                elevation={elevation}
                handleClosed={handleIVDialogClosed}
                handleOpened={handleIVDialogOpened}
            />
            <ChangePasswordDialog
                username={userInfo?.display_name || ""}
                open={dialogPWChangeOpen}
                setClosed={() => {
                    handleResetState();
                }}
            />
            <EditProfileDialog
                key={`${profile?.version || 0}-${profileOpen}`}
                open={profileOpen}
                profile={profile}
                onClose={() => setProfileOpen(false)}
                onSaved={(updated) => (updated ? setProfile(updated) : refreshProfile())}
            />
            <SetPasswordDialog
                open={setPasswordOpen}
                onClose={() => setSetPasswordOpen(false)}
                onSaved={refreshProfile}
            />
            <DisablePasswordDialog
                open={disablePasswordOpen}
                profile={profile}
                onClose={() => setDisablePasswordOpen(false)}
                onSaved={refreshProfile}
            />

            <Container
                sx={{
                    alignItems: "flex-start",
                    display: "flex",
                    height: "100vh",
                    justifyContent: "center",
                    pt: 8,
                }}
            >
                <Paper
                    variant="outlined"
                    sx={{
                        alignItems: "center",
                        display: "flex",
                        height: "auto",
                        justifyContent: "center",
                    }}
                >
                    <Stack spacing={2} sx={{ m: 2, width: "100%" }}>
                        <Box sx={{ p: { md: 3, xs: 1 } }}>
                            <Box
                                sx={{
                                    border: `1px solid ${theme.palette.grey[600]}`,
                                    borderRadius: 1,
                                    mb: 1,
                                    p: 1.25,
                                    width: "100%",
                                }}
                            >
                                <Typography>
                                    {translate("Name")}: {profile?.display_name || userInfo?.display_name || ""}
                                </Typography>
                            </Box>
                            <Box
                                sx={{
                                    border: `1px solid ${theme.palette.grey[600]}`,
                                    borderRadius: 1,
                                    mb: 1,
                                    p: 1.25,
                                    width: "100%",
                                }}
                            >
                                <Box display="flex" alignItems="center">
                                    <Typography sx={{ mr: 1 }}>{translate("Email")}:</Typography>
                                    <Typography>{userInfo?.emails?.[0] || ""}</Typography>
                                </Box>
                                {userInfo?.emails && userInfo.emails.length > 1 && (
                                    <List sx={{ padding: 0, pl: 4, width: "100%" }}>
                                        {" "}
                                        {userInfo.emails.slice(1).map((email: string) => (
                                            <ListItem key={email} sx={{ paddingBottom: 0, paddingTop: 0 }}>
                                                <Typography>{email}</Typography>
                                            </ListItem>
                                        ))}
                                    </List>
                                )}
                            </Box>
                            <Box
                                sx={{ border: `1px solid ${theme.palette.grey[600]}`, borderRadius: 1, mb: 1, p: 1.25 }}
                            >
                                <Typography>
                                    {translate("Password")}:{" "}
                                    {profile?.password_enabled ? "●●●●●●●●" : translate("Not configured")}
                                </Typography>
                            </Box>
                            <Stack spacing={1}>
                                <Button variant="outlined" onClick={() => setProfileOpen(true)}>
                                    {translate("Edit Profile")}
                                </Button>
                                {profile?.password_enabled ? (
                                    <>
                                        <PasswordChangeButton
                                            configuration={configuration}
                                            translate={translate}
                                            handleChangePassword={handleChangePassword}
                                        />
                                        {profile.telegram_linked && (
                                            <Button
                                                color="error"
                                                variant="outlined"
                                                onClick={() => setDisablePasswordOpen(true)}
                                            >
                                                {translate("Disable Password Login")}
                                            </Button>
                                        )}
                                    </>
                                ) : profile?.telegram_linked ? (
                                    <Button
                                        variant="contained"
                                        onClick={() => window.location.assign(getTelegramPasswordProofURL())}
                                    >
                                        {translate("Set Password")}
                                    </Button>
                                ) : null}
                            </Stack>
                            <TelegramAccountLink enabled={getTelegramLogin()} onConnect={handleConnectTelegram} />
                        </Box>
                    </Stack>
                </Paper>
            </Container>
        </Fragment>
    );
};

export default SettingsView;
