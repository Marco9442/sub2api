package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *channelMonitorRepository) ListDisplayGroups(ctx context.Context) ([]service.ChannelMonitorDisplayGroup, error) {
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx, `
		SELECT id, name, sort_order
		FROM channel_monitor_display_groups
		ORDER BY sort_order, id
	`)
	if err != nil {
		return nil, fmt.Errorf("list display groups: %w", err)
	}
	defer rows.Close()

	out := make([]service.ChannelMonitorDisplayGroup, 0)
	for rows.Next() {
		var group service.ChannelMonitorDisplayGroup
		if err := rows.Scan(&group.ID, &group.Name, &group.SortOrder); err != nil {
			return nil, fmt.Errorf("scan display group: %w", err)
		}
		out = append(out, group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list display groups: %w", err)
	}
	return out, nil
}

func (r *channelMonitorRepository) ListDisplayMemberships(ctx context.Context) ([]service.ChannelMonitorDisplayMembership, error) {
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx, `
		SELECT id, name, provider, enabled, group_name, display_sort
		FROM channel_monitors
		ORDER BY display_sort, id
	`)
	if err != nil {
		return nil, fmt.Errorf("list display memberships: %w", err)
	}
	defer rows.Close()

	out := make([]service.ChannelMonitorDisplayMembership, 0)
	for rows.Next() {
		var row service.ChannelMonitorDisplayMembership
		if err := rows.Scan(&row.ID, &row.Name, &row.Provider, &row.Enabled, &row.GroupName, &row.DisplaySort); err != nil {
			return nil, fmt.Errorf("scan display membership: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list display memberships: %w", err)
	}
	return out, nil
}

func (r *channelMonitorRepository) NextDisplaySort(ctx context.Context, groupName string) (int, error) {
	var next int
	err := scanSingleRow(ctx, clientFromContext(ctx, r.client), `
		SELECT COALESCE(MAX(display_sort), -1) + 1
		FROM channel_monitors
		WHERE group_name = $1
	`, []any{groupName}, &next)
	if err != nil {
		return 0, fmt.Errorf("next display sort: %w", err)
	}
	return next, nil
}

func (r *channelMonitorRepository) CreateDisplayGroup(ctx context.Context, name string) (*service.ChannelMonitorDisplayGroup, error) {
	var group service.ChannelMonitorDisplayGroup
	err := scanSingleRow(ctx, clientFromContext(ctx, r.client), `
		INSERT INTO channel_monitor_display_groups (name, sort_order)
		SELECT $1, COALESCE((SELECT MAX(sort_order) FROM channel_monitor_display_groups), -1) + 1
		WHERE (SELECT COUNT(*) FROM channel_monitor_display_groups) < $2
		RETURNING id, name, sort_order
	`, []any{name, service.MaxChannelMonitorDisplayGroups}, &group.ID, &group.Name, &group.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrChannelMonitorDisplayGroupInvalid
	}
	if err != nil {
		return nil, translatePersistenceError(err, nil, service.ErrChannelMonitorDisplayGroupNameTaken)
	}
	return &group, nil
}

func (r *channelMonitorRepository) RenameDisplayGroup(ctx context.Context, id int64, name string) error {
	client := clientFromContext(ctx, r.client)
	var oldName string
	err := scanSingleRow(ctx, client, `
		SELECT name FROM channel_monitor_display_groups WHERE id = $1
	`, []any{id}, &oldName)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrChannelMonitorDisplayGroupNotFound
	}
	if err != nil {
		return fmt.Errorf("load display group: %w", err)
	}
	if _, err := client.ExecContext(ctx, `
		UPDATE channel_monitor_display_groups
		SET name = $1, updated_at = NOW()
		WHERE id = $2
	`, name, id); err != nil {
		return translatePersistenceError(err, service.ErrChannelMonitorDisplayGroupNotFound, service.ErrChannelMonitorDisplayGroupNameTaken)
	}
	if _, err := client.ExecContext(ctx, `
		UPDATE channel_monitors
		SET group_name = $1, updated_at = NOW()
		WHERE group_name = $2
	`, name, oldName); err != nil {
		return fmt.Errorf("rename display group members: %w", err)
	}
	return nil
}

func (r *channelMonitorRepository) DeleteDisplayGroup(ctx context.Context, id int64) (int64, error) {
	client := clientFromContext(ctx, r.client)
	var name string
	err := scanSingleRow(ctx, client, `
		SELECT name FROM channel_monitor_display_groups WHERE id = $1
	`, []any{id}, &name)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, service.ErrChannelMonitorDisplayGroupNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("load display group: %w", err)
	}
	result, err := client.ExecContext(ctx, `
		UPDATE channel_monitors
		SET group_name = '', updated_at = NOW()
		WHERE group_name = $1
	`, name)
	if err != nil {
		return 0, fmt.Errorf("clear display group members: %w", err)
	}
	affected, _ := result.RowsAffected()
	if _, err := client.ExecContext(ctx, `
		DELETE FROM channel_monitor_display_groups WHERE id = $1
	`, id); err != nil {
		return 0, fmt.Errorf("delete display group: %w", err)
	}
	return affected, nil
}

func (r *channelMonitorRepository) ApplyDisplayLayout(ctx context.Context, plan service.ChannelMonitorDisplayLayoutPlan) error {
	client := clientFromContext(ctx, r.client)
	if len(plan.Groups) > 0 {
		ids := make([]int64, len(plan.Groups))
		sorts := make([]int64, len(plan.Groups))
		for i, group := range plan.Groups {
			ids[i] = group.GroupID
			sorts[i] = int64(group.SortOrder)
		}
		if _, err := client.ExecContext(ctx, `
			UPDATE channel_monitor_display_groups AS g
			SET sort_order = u.sort, updated_at = NOW()
			FROM unnest($1::bigint[], $2::bigint[]) AS u(id, sort)
			WHERE g.id = u.id
		`, pq.Array(ids), pq.Array(sorts)); err != nil {
			return fmt.Errorf("update display group order: %w", err)
		}
	}
	for _, group := range plan.Groups {
		if err := assignDisplayMembers(ctx, client, group.Name, group.MonitorIDs); err != nil {
			return err
		}
	}
	if err := assignDisplayMembers(ctx, client, "", plan.UngroupedIDs); err != nil {
		return err
	}
	return nil
}

func assignDisplayMembers(ctx context.Context, client interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, groupName string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	sorts := make([]int64, len(ids))
	for i := range ids {
		sorts[i] = int64(i)
	}
	if _, err := client.ExecContext(ctx, `
		UPDATE channel_monitors AS m
		SET group_name = $1, display_sort = u.sort, updated_at = NOW()
		FROM unnest($2::bigint[], $3::bigint[]) AS u(id, sort)
		WHERE m.id = u.id
	`, groupName, pq.Array(ids), pq.Array(sorts)); err != nil {
		return fmt.Errorf("assign display group members: %w", err)
	}
	return nil
}
