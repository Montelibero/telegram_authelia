import { useEffect } from "react";

import { Route, Routes, useLocation } from "react-router-dom";

import {
    AdminGroupsSubRoute,
    AdminPendingSubRoute,
    AdminPermissionsSubRoute,
    AdminUsersSubRoute,
    IndexRoute,
    SecuritySubRoute,
    SettingsRoute,
    SettingsTwoFactorAuthenticationSubRoute,
} from "@constants/Routes";
import { useRouterNavigate } from "@hooks/RouterNavigate";
import { useAutheliaState } from "@hooks/State";
import SettingsLayout from "@layouts/SettingsLayout";
import { AuthenticationLevel } from "@services/State";
import GroupsView from "@views/Settings/Admin/GroupsView";
import PendingView from "@views/Settings/Admin/PendingView";
import PermissionsView from "@views/Settings/Admin/PermissionsView";
import UsersView from "@views/Settings/Admin/UsersView";
import SecurityView from "@views/Settings/Security/SecurityView";
import SettingsView from "@views/Settings/SettingsView";
import TwoFactorAuthenticationView from "@views/Settings/TwoFactorAuthentication/TwoFactorAuthenticationView";

const SettingsRouter = function () {
    const navigate = useRouterNavigate();
    const location = useLocation();
    const [state, fetchState, , fetchStateError] = useAutheliaState();

    useEffect(() => {
        fetchState();
    }, [fetchState]);

    useEffect(() => {
        if (fetchStateError || (state && state.authentication_level < AuthenticationLevel.OneFactor)) {
            navigate(IndexRoute);
        }
    }, [state, fetchStateError, navigate]);

    useEffect(() => {
        const delegatedAdminRoute = [AdminUsersSubRoute, AdminPendingSubRoute, AdminGroupsSubRoute].some((route) =>
            location.pathname.endsWith(route),
        );
        const fullAdminRoute = location.pathname.endsWith(AdminPermissionsSubRoute);
        if (
            state &&
            ((delegatedAdminRoute && !state.administrator && !state.manager) ||
                (fullAdminRoute && !state.administrator))
        ) {
            navigate(SettingsRoute);
        }
    }, [location.pathname, navigate, state]);

    return (
        <SettingsLayout administrator={Boolean(state?.administrator)} manager={Boolean(state?.manager)}>
            <Routes>
                <Route path={IndexRoute} element={<SettingsView />} />
                <Route path={SecuritySubRoute} element={<SecurityView />} />
                <Route path={SettingsTwoFactorAuthenticationSubRoute} element={<TwoFactorAuthenticationView />} />
                {state?.administrator || state?.manager ? (
                    <Route
                        path={AdminUsersSubRoute}
                        element={
                            <UsersView
                                currentUsername={state.username}
                                fullAdministrator={Boolean(state.administrator)}
                            />
                        }
                    />
                ) : null}
                {state?.administrator || state?.manager ? (
                    <Route
                        path={AdminPendingSubRoute}
                        element={<PendingView fullAdministrator={Boolean(state.administrator)} />}
                    />
                ) : null}
                {state?.administrator || state?.manager ? (
                    <Route
                        path={AdminGroupsSubRoute}
                        element={
                            <GroupsView
                                currentUsername={state.username}
                                fullAdministrator={Boolean(state.administrator)}
                            />
                        }
                    />
                ) : null}
                {state?.administrator ? <Route path={AdminPermissionsSubRoute} element={<PermissionsView />} /> : null}
            </Routes>
        </SettingsLayout>
    );
};

export default SettingsRouter;
