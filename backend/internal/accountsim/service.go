package accountsim

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// tickInterval 是后台补齐的检查间隔。演进只处理已结束的 Step，检查得更勤不会多生成数据。
const tickInterval = time.Minute

// stateStore 是服务需要的持久化能力，Store 实现它，测试可替换为内存实现。
type stateStore interface {
	Load(ctx context.Context) (*State, Config, error)
	SaveState(ctx context.Context, state *State) error
	SaveConfig(ctx context.Context, cfg Config) error
}

// Service 持有模拟状态，后台持续演进，并为接口层提供读写。
// 状态以内存为准、每次变更后整体落库，按单实例部署设计。
type Service struct {
	store stateStore
	// now 是时间来源，测试可以固定或推进时间。
	now func() time.Time

	mu     sync.RWMutex
	state  *State
	config Config
	loaded bool

	stop chan struct{}
	done chan struct{}
}

// NewService 创建服务，不立即读库；第一次使用或 Start 时加载。
func NewService(store stateStore, now func() time.Time) *Service {
	return &Service{store: store, now: now}
}

// Start 启动后台演进。重复调用无效。
func (s *Service) Start() {
	if s.stop != nil {
		return
	}
	s.stop, s.done = make(chan struct{}), make(chan struct{})
	go s.run()
}

// Stop 停止后台演进并等待当前一轮结束。
func (s *Service) Stop() {
	if s.stop == nil {
		return
	}
	close(s.stop)
	<-s.done
}

func (s *Service) run() {
	defer close(s.done)
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		if err := s.Tick(context.Background()); err != nil {
			slog.Warn("account simulation tick failed", "error", err)
		}
		select {
		case <-s.stop:
			return
		case <-ticker.C:
		}
	}
}

// Tick 把状态补齐到当前时刻并落库。启用开关关闭时只加载不演进。
func (s *Service) Tick(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(ctx); err != nil {
		return err
	}
	if !s.config.Enabled {
		return nil
	}
	before := s.state.EvolvedUntil
	Evolve(s.state, s.now(), s.config)
	if s.state.EvolvedUntil.Equal(before) {
		return nil
	}
	return s.store.SaveState(ctx, s.state)
}

func (s *Service) ensureLoaded(ctx context.Context) error {
	if s.loaded {
		return nil
	}
	state, cfg, err := s.store.Load(ctx)
	if err != nil {
		return err
	}
	s.state, s.config, s.loaded = state, cfg, true
	return nil
}

// View 在读锁下访问状态。fn 不得修改状态，也不得保留指针到锁外。
func (s *Service) View(ctx context.Context, fn func(state *State, cfg Config, now time.Time)) error {
	if err := s.load(ctx); err != nil {
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(s.state, s.config, s.now())
	return nil
}

// Update 在副本上修改。落库成功后才替换内存；失败时原状态保持不变。
func (s *Service) Update(ctx context.Context, fn func(state *State, cfg Config, now time.Time) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(ctx); err != nil {
		return err
	}
	next := cloneState(s.state)
	if err := fn(next, s.config, s.now()); err != nil {
		return err
	}
	if err := s.store.SaveState(ctx, next); err != nil {
		return err
	}
	s.state = next
	return nil
}

// Config 返回当前配置。
func (s *Service) Config(ctx context.Context) (Config, error) {
	var out Config
	err := s.View(ctx, func(_ *State, cfg Config, _ time.Time) { out = cfg })
	return out, err
}

// SetConfig 修正并保存配置，只影响之后的演进，不追溯已有账号。
func (s *Service) SetConfig(ctx context.Context, cfg Config) (Config, error) {
	cfg = cfg.Normalize()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(ctx); err != nil {
		return Config{}, err
	}
	if err := s.store.SaveConfig(ctx, cfg); err != nil {
		return Config{}, err
	}
	s.config = cfg
	return cfg, nil
}

func (s *Service) load(ctx context.Context) error {
	s.mu.RLock()
	loaded := s.loaded
	s.mu.RUnlock()
	if loaded {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureLoaded(ctx)
}
