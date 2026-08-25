package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/storage"
)

type storageGroupStore interface {
	ListMTLAdminGroups(ctx context.Context) ([]model.MTLAdminGroupSummary, error)
	LoadMTLAdminGroup(ctx context.Context, name string) (model.MTLAdminGroupDetails, bool, error)
	CreateMTLAdminGroup(ctx context.Context, name, actor string) (model.MTLAdminGroupDetails, error)
	RenameMTLAdminGroup(ctx context.Context, name, newName string, expectedVersion int, actor string) (model.MTLAdminGroupDetails, []string, error)
	DeleteMTLAdminGroup(ctx context.Context, name string, expectedVersion int, actor string) ([]string, error)
	AddMTLAdminGroupUser(ctx context.Context, name, username string, expectedVersion int, actor string) (model.MTLAdminGroupDetails, error)
	RemoveMTLAdminGroupUser(ctx context.Context, name, username string, expectedVersion int, actor string) (model.MTLAdminGroupDetails, error)
}

func newStorageGroupCmd(ctx *CmdCtx) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "group",
		Short:   "Bootstrap and recover SQL-backed groups",
		Long:    "List, inspect, create, rename, delete, and repair memberships for SQL-backed groups.",
		Example: "authelia storage group --help\nauthelia storage group list",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		DisableAutoGenTag: true,
	}
	cmd.AddCommand(
		newStorageGroupListCmd(ctx),
		newStorageGroupShowCmd(ctx),
		newStorageGroupCreateCmd(ctx),
		newStorageGroupRenameCmd(ctx),
		newStorageGroupDeleteCmd(ctx),
		newStorageGroupUserCmd(ctx, true),
		newStorageGroupUserCmd(ctx, false),
	)
	return cmd
}

func newStorageGroupListCmd(ctx *CmdCtx) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List groups", Example: "authelia storage group list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return runStorageGroupList(cmd.Context(), cmd.OutOrStdout(), ctx.providers.StorageProvider)
	}, DisableAutoGenTag: true}
}

func newStorageGroupShowCmd(ctx *CmdCtx) *cobra.Command {
	return &cobra.Command{Use: "show GROUP", Short: "Show a group and its users", Example: "authelia storage group show admins", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return runStorageGroupShow(cmd.Context(), cmd.OutOrStdout(), ctx.providers.StorageProvider, args[0])
	}, DisableAutoGenTag: true}
}

func newStorageGroupCreateCmd(ctx *CmdCtx) *cobra.Command {
	cmd := &cobra.Command{Use: "create GROUP", Short: "Create a group", Example: "authelia storage group create admins --actor admin", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		actor, _ := cmd.Flags().GetString("actor")
		_, err := runStorageGroupCreate(cmd.Context(), cmd.OutOrStdout(), ctx.providers.StorageProvider, args[0], actor)
		return err
	}, DisableAutoGenTag: true}
	cmd.Flags().String("actor", "", "existing local username performing the operation")
	return cmd
}

func newStorageGroupRenameCmd(ctx *CmdCtx) *cobra.Command {
	cmd := &cobra.Command{Use: "rename GROUP NEW_GROUP", Short: "Rename a group", Example: "authelia storage group rename old new --version 2 --actor admin", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		version, _ := cmd.Flags().GetInt("version")
		actor, _ := cmd.Flags().GetString("actor")
		_, _, err := runStorageGroupRename(cmd.Context(), cmd.OutOrStdout(), ctx.providers.StorageProvider, args[0], args[1], version, actor)
		return err
	}, DisableAutoGenTag: true}
	addStorageGroupMutationFlags(cmd)
	return cmd
}

func newStorageGroupDeleteCmd(ctx *CmdCtx) *cobra.Command {
	cmd := &cobra.Command{Use: "delete GROUP", Short: "Delete a group", Example: "authelia storage group delete obsolete --version 3 --actor admin", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		version, _ := cmd.Flags().GetInt("version")
		actor, _ := cmd.Flags().GetString("actor")
		_, err := runStorageGroupDelete(cmd.Context(), cmd.OutOrStdout(), ctx.providers.StorageProvider, args[0], version, actor)
		return err
	}, DisableAutoGenTag: true}
	addStorageGroupMutationFlags(cmd)
	return cmd
}

func newStorageGroupUserCmd(ctx *CmdCtx, add bool) *cobra.Command {
	use, short, example := "remove-user GROUP USER", "Remove a user from a group", "authelia storage group remove-user admins bublik --version 2 --actor admin"
	if add {
		use, short, example = "add-user GROUP USER", "Add a user to a group", "authelia storage group add-user admins bublik --version 1 --actor admin"
	}
	cmd := &cobra.Command{Use: use, Short: short, Example: example, Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		version, _ := cmd.Flags().GetInt("version")
		actor, _ := cmd.Flags().GetString("actor")
		if add {
			_, err := runStorageGroupAddUser(cmd.Context(), cmd.OutOrStdout(), ctx.providers.StorageProvider, args[0], args[1], version, actor)
			return err
		}
		_, err := runStorageGroupRemoveUser(cmd.Context(), cmd.OutOrStdout(), ctx.providers.StorageProvider, args[0], args[1], version, actor)
		return err
	}, DisableAutoGenTag: true}
	addStorageGroupMutationFlags(cmd)
	return cmd
}

func addStorageGroupMutationFlags(cmd *cobra.Command) {
	cmd.Flags().Int("version", 0, "group version shown by list or show")
	cmd.Flags().String("actor", "", "existing local username performing the operation")
	_ = cmd.MarkFlagRequired("version")
}

func groupStore(provider storage.Provider) (storageGroupStore, error) {
	store, ok := provider.(storageGroupStore)
	if !ok {
		return nil, errors.New("configured storage provider is not compatible with SQL-backed groups")
	}
	return store, nil
}

func runStorageGroupList(ctx context.Context, w io.Writer, provider storage.Provider) error {
	store, err := groupStore(provider)
	if err != nil {
		return err
	}
	groups, err := store.ListMTLAdminGroups(ctx)
	if err != nil {
		return err
	}
	if len(groups) == 0 {
		_, _ = fmt.Fprintln(w, "No groups.")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "GROUP\tVERSION\tUSERS")
	for _, group := range groups {
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%d\n", group.Name, group.Version, group.UserCount)
	}
	return tw.Flush()
}

func runStorageGroupShow(ctx context.Context, w io.Writer, provider storage.Provider, name string) error {
	store, err := groupStore(provider)
	if err != nil {
		return err
	}
	group, found, err := store.LoadMTLAdminGroup(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		return storage.ErrMTLGroupNotFound
	}
	_, _ = fmt.Fprintf(w, "Group: %s\nVersion: %d\nUsers: %s\n", group.Name, group.Version, strings.Join(group.Users, ", "))
	return nil
}

func runStorageGroupCreate(ctx context.Context, w io.Writer, provider storage.Provider, name, actor string) (model.MTLAdminGroupDetails, error) {
	store, err := groupStore(provider)
	if err != nil {
		return model.MTLAdminGroupDetails{}, err
	}
	group, err := store.CreateMTLAdminGroup(ctx, name, actor)
	if err == nil {
		_, _ = fmt.Fprintf(w, "Created group %s at version %d.\n", group.Name, group.Version)
	}
	return group, err
}

func runStorageGroupRename(ctx context.Context, w io.Writer, provider storage.Provider, name, newName string, version int, actor string) (model.MTLAdminGroupDetails, []string, error) {
	store, err := groupStore(provider)
	if err != nil {
		return model.MTLAdminGroupDetails{}, nil, err
	}
	group, affected, err := store.RenameMTLAdminGroup(ctx, name, newName, version, actor)
	if err == nil {
		_, _ = fmt.Fprintf(w, "Renamed group %s to %s. Affected users: %s. External YAML ACL references are not updated.\n", name, newName, strings.Join(affected, ", "))
	}
	return group, affected, err
}

func runStorageGroupDelete(ctx context.Context, w io.Writer, provider storage.Provider, name string, version int, actor string) ([]string, error) {
	store, err := groupStore(provider)
	if err != nil {
		return nil, err
	}
	affected, err := store.DeleteMTLAdminGroup(ctx, name, version, actor)
	if err == nil {
		_, _ = fmt.Fprintf(w, "Deleted group %s. Affected users: %s. External YAML ACL references are not updated.\n", name, strings.Join(affected, ", "))
	}
	return affected, err
}

func runStorageGroupAddUser(ctx context.Context, w io.Writer, provider storage.Provider, name, username string, version int, actor string) (model.MTLAdminGroupDetails, error) {
	store, err := groupStore(provider)
	if err != nil {
		return model.MTLAdminGroupDetails{}, err
	}
	group, err := store.AddMTLAdminGroupUser(ctx, name, username, version, actor)
	if err == nil {
		_, _ = fmt.Fprintf(w, "Added user %s to group %s at version %d.\n", username, name, group.Version)
	}
	return group, err
}

func runStorageGroupRemoveUser(ctx context.Context, w io.Writer, provider storage.Provider, name, username string, version int, actor string) (model.MTLAdminGroupDetails, error) {
	store, err := groupStore(provider)
	if err != nil {
		return model.MTLAdminGroupDetails{}, err
	}
	group, err := store.RemoveMTLAdminGroupUser(ctx, name, username, version, actor)
	if err == nil {
		_, _ = fmt.Fprintf(w, "Removed user %s from group %s at version %d.\n", username, name, group.Version)
	}
	return group, err
}
