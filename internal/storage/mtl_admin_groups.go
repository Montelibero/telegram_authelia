package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/authelia/authelia/v4/internal/model"
)

var (
	ErrMTLGroupNotFound      = errors.New("MTL group not found")
	ErrMTLMembershipNotFound = errors.New("MTL group membership not found")
)

// ReconcileMTLGroups creates configured groups which do not exist yet. Existing groups and memberships are unchanged.
func (p *SQLProvider) ReconcileMTLGroups(ctx context.Context, groups []string) (err error) {
	query := `INSERT INTO mtl_groups (name) VALUES (?) ON CONFLICT(name) DO NOTHING`
	if p.name == providerMySQL {
		query = `INSERT IGNORE INTO mtl_groups (name) VALUES (?)`
	}
	query = p.db.Rebind(query)

	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}

		if _, err = p.db.ExecContext(ctx, query, group); err != nil {
			return fmt.Errorf("failed to create reconciled MTL group %q: %w", group, err)
		}
	}

	return nil
}

// ListMTLAdminGroups returns groups and membership counts in name order.
func (p *SQLProvider) ListMTLAdminGroups(ctx context.Context) (groups []model.MTLAdminGroupSummary, err error) {
	query := `SELECT g.name, g.version, COUNT(gm.user_id) AS user_count, g.updated_at FROM mtl_groups g LEFT JOIN mtl_group_memberships gm ON gm.group_id = g.id GROUP BY g.id, g.name, g.version, g.updated_at ORDER BY g.name`
	if err = p.db.SelectContext(ctx, &groups, query); err != nil {
		return nil, fmt.Errorf("failed to list MTL admin groups: %w", err)
	}
	return groups, nil
}

// LoadMTLAdminGroup returns one group and its members.
func (p *SQLProvider) LoadMTLAdminGroup(ctx context.Context, name string) (details model.MTLAdminGroupDetails, found bool, err error) {
	query := p.db.Rebind(`SELECT g.name, g.version, COUNT(gm.user_id) AS user_count, g.updated_at FROM mtl_groups g LEFT JOIN mtl_group_memberships gm ON gm.group_id = g.id WHERE g.name = ? GROUP BY g.id, g.name, g.version, g.updated_at`)
	if err = p.db.GetContext(ctx, &details.MTLAdminGroupSummary, query, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return details, false, nil
		}
		return details, false, fmt.Errorf("failed to load MTL admin group: %w", err)
	}
	if err = p.db.SelectContext(ctx, &details.Users, p.db.Rebind(`SELECT u.username FROM mtl_users u INNER JOIN mtl_group_memberships gm ON gm.user_id = u.id INNER JOIN mtl_groups g ON g.id = gm.group_id WHERE g.name = ? ORDER BY u.username`), name); err != nil {
		return details, false, fmt.Errorf("failed to load MTL admin group users: %w", err)
	}
	return details, true, nil
}

// CreateMTLAdminGroup creates a group without restricting its name format.
func (p *SQLProvider) CreateMTLAdminGroup(ctx context.Context, name, actor string) (details model.MTLAdminGroupDetails, err error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return details, fmt.Errorf("failed to begin MTL admin group creation: %w", err)
	}
	defer rollbackMTLAdmin(tx, &err)
	actorID, err := loadOptionalMTLActor(ctx, tx, actor)
	if err != nil {
		return details, err
	}
	if _, err = tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_groups (name) VALUES (?)`), name); err != nil {
		return details, mapMTLConflict("failed to create MTL admin group", err)
	}
	if err = auditMTLAdmin(ctx, tx, actorID, "group.created", "group", name); err != nil {
		return details, err
	}
	if err = tx.Commit(); err != nil {
		return details, fmt.Errorf("failed to commit MTL admin group creation: %w", err)
	}
	details, _, err = p.LoadMTLAdminGroup(ctx, name)
	return details, err
}

// RenameMTLAdminGroup renames a group and returns the usernames affected by external ACL references.
func (p *SQLProvider) RenameMTLAdminGroup(ctx context.Context, name, newName string, expectedVersion int, actor string) (details model.MTLAdminGroupDetails, affected []string, err error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return details, nil, fmt.Errorf("failed to begin MTL admin group rename: %w", err)
	}
	defer rollbackMTLAdmin(tx, &err)
	groupID, err := loadMTLAdminGroupVersion(ctx, tx, name, expectedVersion)
	if err != nil {
		return details, nil, err
	}
	if affected, err = loadMTLAdminGroupUsers(ctx, tx, groupID); err != nil {
		return details, nil, err
	}
	if err = bumpMTLAdminGroupUserEpochs(ctx, tx, groupID); err != nil {
		return details, nil, err
	}
	actorID, err := loadOptionalMTLActor(ctx, tx, actor)
	if err != nil {
		return details, nil, err
	}
	result, err := tx.ExecContext(ctx, tx.Rebind(`UPDATE mtl_groups SET name = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND version = ?`), newName, groupID, expectedVersion)
	if err != nil {
		return details, nil, mapMTLConflict("failed to rename MTL admin group", err)
	}
	if err = requireMTLAdminRow(result); err != nil {
		return details, nil, err
	}
	if err = auditMTLAdmin(ctx, tx, actorID, "group.renamed", "group", name); err != nil {
		return details, nil, err
	}
	if err = tx.Commit(); err != nil {
		return details, nil, fmt.Errorf("failed to commit MTL admin group rename: %w", err)
	}
	details, _, err = p.LoadMTLAdminGroup(ctx, newName)
	return details, affected, err
}

// DeleteMTLAdminGroup deletes a group and returns the users whose membership was removed.
func (p *SQLProvider) DeleteMTLAdminGroup(ctx context.Context, name string, expectedVersion int, actor string) (affected []string, err error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin MTL admin group deletion: %w", err)
	}
	defer rollbackMTLAdmin(tx, &err)
	groupID, err := loadMTLAdminGroupVersion(ctx, tx, name, expectedVersion)
	if err != nil {
		return nil, err
	}
	if affected, err = loadMTLAdminGroupUsers(ctx, tx, groupID); err != nil {
		return nil, err
	}
	if err = bumpMTLAdminGroupUserEpochs(ctx, tx, groupID); err != nil {
		return nil, err
	}
	actorID, err := loadOptionalMTLActor(ctx, tx, actor)
	if err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM mtl_groups WHERE id = ? AND version = ?`), groupID, expectedVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to delete MTL admin group: %w", err)
	}
	if err = requireMTLAdminRow(result); err != nil {
		return nil, err
	}
	if err = auditMTLAdmin(ctx, tx, actorID, "group.deleted", "group", name); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit MTL admin group deletion: %w", err)
	}
	return affected, nil
}

// AddMTLAdminGroupUser adds one membership and increments the group version.
func (p *SQLProvider) AddMTLAdminGroupUser(ctx context.Context, name, username string, expectedVersion int, actor string) (model.MTLAdminGroupDetails, error) {
	return p.mutateMTLAdminGroupUser(ctx, name, username, expectedVersion, actor, true)
}

// RemoveMTLAdminGroupUser removes one membership and increments the group version.
func (p *SQLProvider) RemoveMTLAdminGroupUser(ctx context.Context, name, username string, expectedVersion int, actor string) (model.MTLAdminGroupDetails, error) {
	return p.mutateMTLAdminGroupUser(ctx, name, username, expectedVersion, actor, false)
}

func (p *SQLProvider) mutateMTLAdminGroupUser(ctx context.Context, name, username string, expectedVersion int, actor string, add bool) (details model.MTLAdminGroupDetails, err error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return details, fmt.Errorf("failed to begin MTL admin group membership mutation: %w", err)
	}
	defer rollbackMTLAdmin(tx, &err)
	groupID, err := loadMTLAdminGroupVersion(ctx, tx, name, expectedVersion)
	if err != nil {
		return details, err
	}
	var userID int64
	if err = tx.GetContext(ctx, &userID, tx.Rebind(`SELECT id FROM mtl_users WHERE username = ?`), username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return details, ErrMTLUserNotFound
		}
		return details, fmt.Errorf("failed to load MTL admin group user: %w", err)
	}
	actorID, err := loadOptionalMTLActor(ctx, tx, actor)
	if err != nil {
		return details, err
	}
	event := "group.user_added"
	if add {
		if _, err = tx.ExecContext(ctx, tx.Rebind(`INSERT INTO mtl_group_memberships (user_id, group_id) VALUES (?, ?)`), userID, groupID); err != nil {
			return details, mapMTLConflict("failed to add MTL admin group user", err)
		}
	} else {
		event = "group.user_removed"
		result, execErr := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM mtl_group_memberships WHERE user_id = ? AND group_id = ?`), userID, groupID)
		if execErr != nil {
			return details, fmt.Errorf("failed to remove MTL admin group user: %w", execErr)
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return details, fmt.Errorf("failed to check MTL admin group membership removal: %w", rowsErr)
		}
		if rows != 1 {
			return details, ErrMTLMembershipNotFound
		}
	}
	if err = bumpMTLAdminGroupVersion(ctx, tx, groupID, expectedVersion); err != nil {
		return details, err
	}
	if _, err = tx.ExecContext(ctx, tx.Rebind(`UPDATE mtl_users SET session_epoch = session_epoch + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`), userID); err != nil {
		return details, fmt.Errorf("failed to revoke MTL admin group user sessions: %w", err)
	}
	if err = auditMTLAdmin(ctx, tx, actorID, event, "group", name); err != nil {
		return details, err
	}
	if err = tx.Commit(); err != nil {
		return details, fmt.Errorf("failed to commit MTL admin group membership mutation: %w", err)
	}
	details, _, err = p.LoadMTLAdminGroup(ctx, name)
	return details, err
}

func loadMTLAdminGroupVersion(ctx context.Context, tx SQLXTx, name string, expectedVersion int) (int64, error) {
	var row struct {
		ID      int64 `db:"id"`
		Version int   `db:"version"`
	}
	if err := tx.GetContext(ctx, &row, tx.Rebind(`SELECT id, version FROM mtl_groups WHERE name = ?`), name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrMTLGroupNotFound
		}
		return 0, fmt.Errorf("failed to load MTL admin group version: %w", err)
	}
	if row.Version != expectedVersion {
		return 0, ErrMTLVersionConflict
	}
	return row.ID, nil
}

func loadMTLAdminGroupUsers(ctx context.Context, tx SQLXTx, groupID int64) ([]string, error) {
	users := []string{}
	if err := tx.SelectContext(ctx, &users, tx.Rebind(`SELECT u.username FROM mtl_users u INNER JOIN mtl_group_memberships gm ON gm.user_id = u.id WHERE gm.group_id = ? ORDER BY u.username`), groupID); err != nil {
		return nil, fmt.Errorf("failed to load MTL admin group users: %w", err)
	}
	return users, nil
}

func bumpMTLAdminGroupVersion(ctx context.Context, tx SQLXTx, groupID int64, expectedVersion int) error {
	result, err := tx.ExecContext(ctx, tx.Rebind(`UPDATE mtl_groups SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND version = ?`), groupID, expectedVersion)
	if err != nil {
		return fmt.Errorf("failed to bump MTL admin group version: %w", err)
	}
	return requireMTLAdminRow(result)
}

func bumpMTLAdminGroupUserEpochs(ctx context.Context, tx SQLXTx, groupID int64) error {
	if _, err := tx.ExecContext(ctx, tx.Rebind(`UPDATE mtl_users SET session_epoch = session_epoch + 1, updated_at = CURRENT_TIMESTAMP WHERE id IN (SELECT user_id FROM mtl_group_memberships WHERE group_id = ?)`), groupID); err != nil {
		return fmt.Errorf("failed to revoke MTL admin group sessions: %w", err)
	}
	return nil
}
