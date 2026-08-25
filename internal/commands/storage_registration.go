package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/storage"
)

type storageRegistrationStore interface {
	LoadMTLRegistration(ctx context.Context, id int64) (model.MTLRegistrationRequest, bool, error)
	ListMTLRegistrations(ctx context.Context, status model.MTLRegistrationStatus) ([]model.MTLRegistrationRequest, error)
	ApproveMTLRegistration(ctx context.Context, approval model.MTLRegistrationApproval) (string, error)
	RejectMTLRegistration(ctx context.Context, id int64, expectedVersion int, actorUsername string) (model.MTLRegistrationRequest, error)
}

func newStorageRegistrationCmd(ctx *CmdCtx) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "registration",
		Short:   "Review external registration requests",
		Long:    "List, inspect, approve, or reject external registration requests stored in SQL.",
		Example: "authelia storage registration --help\nauthelia storage registration list",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		DisableAutoGenTag: true,
	}
	cmd.AddCommand(
		newStorageRegistrationListCmd(ctx),
		newStorageRegistrationShowCmd(ctx),
		newStorageRegistrationApproveCmd(ctx),
		newStorageRegistrationRejectCmd(ctx),
	)
	return cmd
}

func newStorageRegistrationListCmd(ctx *CmdCtx) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List registration requests",
		Example: "authelia storage registration list\nauthelia storage registration list --status all",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			status, _ := cmd.Flags().GetString("status")
			return runStorageRegistrationList(cmd.Context(), cmd.OutOrStdout(), ctx.providers.StorageProvider, status)
		},
		DisableAutoGenTag: true,
	}
	cmd.Flags().String("status", "pending", "request status: pending, approved, rejected, or all")
	return cmd
}

func newStorageRegistrationShowCmd(ctx *CmdCtx) *cobra.Command {
	return &cobra.Command{
		Use:     "show REQUEST_ID",
		Short:   "Show a registration request",
		Example: "authelia storage registration show 42",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseRegistrationID(args[0])
			if err != nil {
				return err
			}
			return runStorageRegistrationShow(cmd.Context(), cmd.OutOrStdout(), ctx.providers.StorageProvider, id)
		},
		DisableAutoGenTag: true,
	}
}

func newStorageRegistrationApproveCmd(ctx *CmdCtx) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "approve REQUEST_ID",
		Short:   "Approve a pending registration and create its user",
		Example: "authelia storage registration approve 42 --version 1\nauthelia storage registration approve 42 --version 1 --username bublik --email bublik@eurmtl.me",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseRegistrationID(args[0])
			if err != nil {
				return err
			}
			version, _ := cmd.Flags().GetInt("version")
			username, _ := cmd.Flags().GetString("username")
			email, _ := cmd.Flags().GetString("email")
			actor, _ := cmd.Flags().GetString("actor")
			return runStorageRegistrationApprove(cmd.Context(), cmd.OutOrStdout(), ctx.providers.StorageProvider, model.MTLRegistrationApproval{RequestID: id, ExpectedVersion: version, Username: username, Email: email, ActorUsername: actor})
		},
		DisableAutoGenTag: true,
	}
	cmd.Flags().Int("version", 0, "request version shown by list or show")
	cmd.Flags().String("username", "", "local username override")
	cmd.Flags().String("email", "", "primary email override")
	cmd.Flags().String("actor", "", "existing local username performing the approval")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

func newStorageRegistrationRejectCmd(ctx *CmdCtx) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reject REQUEST_ID",
		Short:   "Reject a pending registration",
		Example: "authelia storage registration reject 42 --version 1",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseRegistrationID(args[0])
			if err != nil {
				return err
			}
			version, _ := cmd.Flags().GetInt("version")
			actor, _ := cmd.Flags().GetString("actor")
			return runStorageRegistrationReject(cmd.Context(), cmd.OutOrStdout(), ctx.providers.StorageProvider, id, version, actor)
		},
		DisableAutoGenTag: true,
	}
	cmd.Flags().Int("version", 0, "request version shown by list or show")
	cmd.Flags().String("actor", "", "existing local username performing the rejection")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

func registrationStore(provider storage.Provider) (storageRegistrationStore, error) {
	store, ok := provider.(storageRegistrationStore)
	if !ok {
		return nil, errors.New("configured storage provider is not compatible with registration requests")
	}
	return store, nil
}

func runStorageRegistrationList(ctx context.Context, w io.Writer, provider storage.Provider, statusValue string) error {
	store, err := registrationStore(provider)
	if err != nil {
		return err
	}
	var status model.MTLRegistrationStatus
	if statusValue != "all" {
		status = model.MTLRegistrationStatus(statusValue)
		if !status.Valid() {
			return fmt.Errorf("invalid registration status %q", statusValue)
		}
	}
	requests, err := store.ListMTLRegistrations(ctx, status)
	if err != nil {
		return err
	}
	if len(requests) == 0 {
		_, _ = fmt.Fprintln(w, "No registration requests.")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tSTATUS\tVERSION\tPROVIDER\tPROVIDER USERNAME\tPROPOSED USERNAME\tPROPOSED EMAIL")
	for _, request := range requests {
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%d\t%s\t%s\t%s\t%s\n", request.ID, request.Status, request.Version, request.Provider, request.ProviderUsername.String, request.ProposedUsername.String, request.ProposedEmail.String)
	}
	return tw.Flush()
}

func runStorageRegistrationShow(ctx context.Context, w io.Writer, provider storage.Provider, id int64) error {
	store, err := registrationStore(provider)
	if err != nil {
		return err
	}
	request, found, err := store.LoadMTLRegistration(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return storage.ErrMTLRegistrationNotFound
	}
	_, _ = fmt.Fprintf(w, "ID: %d\nStatus: %s\nVersion: %d\nProvider: %s\nProvider user ID: %s\nProvider username: %s\nDisplay name: %s\nProposed username: %s\nProposed email: %s\nRequested at: %s\nUpdated at: %s\n", request.ID, request.Status, request.Version, request.Provider, request.ProviderUserID, request.ProviderUsername.String, request.DisplayName.String, request.ProposedUsername.String, request.ProposedEmail.String, request.RequestedAt.Format("2006-01-02T15:04:05Z07:00"), request.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
	return nil
}

func runStorageRegistrationApprove(ctx context.Context, w io.Writer, provider storage.Provider, approval model.MTLRegistrationApproval) error {
	store, err := registrationStore(provider)
	if err != nil {
		return err
	}
	username, err := store.ApproveMTLRegistration(ctx, approval)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "Approved registration %d and created user %s.\n", approval.RequestID, username)
	return nil
}

func runStorageRegistrationReject(ctx context.Context, w io.Writer, provider storage.Provider, id int64, version int, actor string) error {
	store, err := registrationStore(provider)
	if err != nil {
		return err
	}
	if _, err = store.RejectMTLRegistration(ctx, id, version, actor); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "Rejected registration %d.\n", id)
	return nil
}

func parseRegistrationID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid registration request ID %q", value)
	}
	return id, nil
}
