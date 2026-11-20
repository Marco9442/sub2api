//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func newDisplayGroupRepo(t *testing.T) (*channelMonitorRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return &channelMonitorRepository{client: client, db: db}, mock
}

func TestCreateDisplayGroupTranslatesUniqueViolation(t *testing.T) {
	repo, mock := newDisplayGroupRepo(t)
	mock.ExpectQuery(`INSERT INTO channel_monitor_display_groups`).
		WithArgs("alpha", service.MaxChannelMonitorDisplayGroups).
		WillReturnError(&pq.Error{Code: "23505"})

	group, err := repo.CreateDisplayGroup(context.Background(), "alpha")

	require.Nil(t, group)
	require.ErrorIs(t, err, service.ErrChannelMonitorDisplayGroupNameTaken)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRenameDisplayGroupMissingIDIsNotMonitorNotFound(t *testing.T) {
	repo, mock := newDisplayGroupRepo(t)
	mock.ExpectQuery(`SELECT name FROM channel_monitor_display_groups WHERE id = \$1`).
		WithArgs(int64(9)).
		WillReturnError(sql.ErrNoRows)

	err := repo.RenameDisplayGroup(context.Background(), 9, "beta")

	require.ErrorIs(t, err, service.ErrChannelMonitorDisplayGroupNotFound)
	require.NotErrorIs(t, err, service.ErrChannelMonitorNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreatePersistsDisplaySort(t *testing.T) {
	repo, mock := newDisplayGroupRepo(t)
	mock.ExpectQuery(`INSERT INTO "channel_monitors"[\s\S]*"display_sort"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(41)))

	err := repo.Create(context.Background(), &service.ChannelMonitor{
		Name:            "probe",
		Provider:        service.MonitorProviderOpenAI,
		Endpoint:        "https://example.com",
		APIKey:          "cipher",
		PrimaryModel:    "gpt-5.4-mini",
		GroupName:       "alpha",
		DisplaySort:     4,
		IntervalSeconds: 60,
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdatePersistsDisplaySort(t *testing.T) {
	repo, mock := newDisplayGroupRepo(t)
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "channel_monitors" SET[\s\S]*"display_sort"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .* FROM "channel_monitors" WHERE "id" = \$1`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "created_at", "updated_at", "name", "provider", "check_mode", "account_id",
			"api_mode", "endpoint", "api_key_encrypted", "primary_model", "extra_models",
			"group_name", "display_sort", "enabled", "interval_seconds", "jitter_seconds",
			"last_checked_at", "created_by", "template_id", "extra_headers", "body_override_mode", "body_override",
		}).AddRow(
			int64(7), now, now, "probe", "openai", "probe", nil,
			"chat_completions", "https://example.com", "cipher", "gpt-5.4-mini", []byte("[]"),
			"alpha", 2, false, 60, 0,
			nil, int64(0), nil, []byte("{}"), "off", nil,
		))
	mock.ExpectCommit()

	err := repo.Update(context.Background(), &service.ChannelMonitor{
		ID:              7,
		Name:            "probe",
		Provider:        service.MonitorProviderOpenAI,
		Endpoint:        "https://example.com",
		APIKey:          "cipher",
		PrimaryModel:    "gpt-5.4-mini",
		GroupName:       "alpha",
		DisplaySort:     2,
		IntervalSeconds: 60,
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
