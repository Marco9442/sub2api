package accountsim

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

// Store 把模拟状态读写到独立的 account_sim_* 表，不触碰任何真实业务表。
type Store struct {
	db *sql.DB
}

// NewStore 返回基于给定连接的存储。
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Load 读出状态与配置。没有保存过状态时返回空状态，EvolvedUntil 为零值，演进会按首次使用回填。
// 没有保存过配置时返回默认配置。
func (s *Store) Load(ctx context.Context) (*State, Config, error) {
	state := NewState()
	cfg := DefaultConfig()
	if err := s.loadMeta(ctx, "config", &cfg); err != nil {
		return nil, Config{}, err
	}
	if err := s.loadMeta(ctx, "state", state); err != nil {
		return nil, Config{}, err
	}
	if state.Seq == nil {
		state.Seq = map[string]int64{}
	}
	state.Accounts = map[int64]*Account{}
	if state.EvolvedUntil.IsZero() {
		// 状态整体丢失时残留的账号行不可信，按首次使用重新生成。
		return state, cfg.Normalize(), nil
	}

	rows, err := s.db.QueryContext(ctx, `SELECT data FROM account_sim_accounts`)
	if err != nil {
		return nil, Config{}, fmt.Errorf("load simulated accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, Config{}, fmt.Errorf("scan simulated account: %w", err)
		}
		account := &Account{}
		if err := json.Unmarshal(raw, account); err != nil {
			return nil, Config{}, fmt.Errorf("decode simulated account: %w", err)
		}
		state.Accounts[account.ID] = account
	}
	if err := rows.Err(); err != nil {
		return nil, Config{}, fmt.Errorf("load simulated accounts: %w", err)
	}
	return state, cfg.Normalize(), nil
}

func (s *Store) loadMeta(ctx context.Context, key string, out any) error {
	var raw []byte
	err := s.db.QueryRowContext(ctx, `SELECT value FROM account_sim_meta WHERE key = $1`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load simulation %s: %w", key, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode simulation %s: %w", key, err)
	}
	return nil
}

// SaveState 在一个事务里保存全部账号与状态，并删除状态中已不存在的账号行。
func (s *Store) SaveState(ctx context.Context, state *State) error {
	meta, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode simulation state: %w", err)
	}
	ids := make([]int64, 0, len(state.Accounts))
	payloads := make([]string, 0, len(state.Accounts))
	for _, id := range sortedIDs(state.Accounts) {
		raw, err := json.Marshal(state.Accounts[id])
		if err != nil {
			return fmt.Errorf("encode simulated account %d: %w", id, err)
		}
		ids = append(ids, id)
		payloads = append(payloads, string(raw))
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin simulation save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM account_sim_accounts WHERE NOT (id = ANY($1))`, pq.Array(ids)); err != nil {
		return fmt.Errorf("prune simulated accounts: %w", err)
	}
	if len(ids) > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO account_sim_accounts (id, data, updated_at)
			SELECT id, data::jsonb, NOW() FROM UNNEST($1::bigint[], $2::text[]) AS t(id, data)
			ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data, updated_at = NOW()`,
			pq.Array(ids), pq.Array(payloads)); err != nil {
			return fmt.Errorf("save simulated accounts: %w", err)
		}
	}
	if err := saveMeta(ctx, tx, "state", meta); err != nil {
		return err
	}
	return tx.Commit()
}

// SaveConfig 保存配置。
func (s *Store) SaveConfig(ctx context.Context, cfg Config) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode simulation config: %w", err)
	}
	return saveMeta(ctx, s.db, "config", raw)
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func saveMeta(ctx context.Context, db execer, key string, value []byte) error {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO account_sim_meta (key, value, updated_at) VALUES ($1, $2::jsonb, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, key, string(value)); err != nil {
		return fmt.Errorf("save simulation %s: %w", key, err)
	}
	return nil
}
