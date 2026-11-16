//go:build integration

package accountsim

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
)

// openTestStore 在独立 schema 里建表，测试结束删除。DSN 来自 ACCOUNTSIM_TEST_DSN，未设置时跳过。
func openTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("ACCOUNTSIM_TEST_DSN")
	if dsn == "" {
		t.Skip("ACCOUNTSIM_TEST_DSN 未设置")
	}
	ctx := context.Background()
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("accountsim_test_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close()
	})
	db, err := sql.Open("postgres", dsn+"&search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ddl, err := migrations.FS.ReadFile("251_account_sim_tables.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(ddl)); err != nil {
		t.Fatal(err)
	}
	return NewStore(db)
}

func TestStoreRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	cfg := DefaultConfig()
	cfg.DailyNewMax = 80
	if err := store.SaveConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	state := NewState()
	Evolve(state, time.Unix(idAnchorUnix, 0), cfg)
	if err := store.SaveState(ctx, state); err != nil {
		t.Fatal(err)
	}
	for id := range state.Accounts {
		delete(state.Accounts, id)
		break
	}
	if err := store.SaveState(ctx, state); err != nil {
		t.Fatal(err)
	}

	loaded, loadedCfg, err := store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loadedCfg.DailyNewMax != 80 {
		t.Fatalf("配置未持久化: %d", loadedCfg.DailyNewMax)
	}
	if len(loaded.Accounts) != len(state.Accounts) || loaded.LastID != state.LastID {
		t.Fatalf("状态未完整恢复: %d/%d 个账号, LastID %d/%d", len(loaded.Accounts), len(state.Accounts), loaded.LastID, state.LastID)
	}
	if !loaded.EvolvedUntil.Equal(state.EvolvedUntil) || len(loaded.Groups) != len(state.Groups) {
		t.Fatal("演进游标或分组未恢复")
	}
}

func TestStoreEmptyMeansFirstUse(t *testing.T) {
	store := openTestStore(t)
	state, cfg, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !state.EvolvedUntil.IsZero() || len(state.Accounts) != 0 {
		t.Fatal("空库应视为首次使用")
	}
	if cfg.DailyNewMin != DefaultConfig().DailyNewMin {
		t.Fatal("空库应返回默认配置")
	}
}
