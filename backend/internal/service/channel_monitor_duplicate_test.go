//go:build unit

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

type duplicateChannelMonitorRepoStub struct {
	ChannelMonitorRepository
	source      *ChannelMonitor
	created     []*ChannelMonitor
	byOperation map[string]*ChannelMonitor
	nextID      int64
}

func (r *duplicateChannelMonitorRepoStub) GetByID(_ context.Context, id int64) (*ChannelMonitor, error) {
	if r.source == nil || r.source.ID != id {
		return nil, ErrChannelMonitorNotFound
	}
	return r.source, nil
}

func (r *duplicateChannelMonitorRepoStub) Create(_ context.Context, monitor *ChannelMonitor) error {
	r.nextID++
	monitor.ID = 100 + r.nextID
	monitor.CreatedAt = time.Date(2026, time.July, 16, 8, 0, 0, 0, time.UTC)
	monitor.UpdatedAt = monitor.CreatedAt

	stored := *monitor
	stored.ExtraModels = append([]string(nil), monitor.ExtraModels...)
	stored.ExtraHeaders = cloneStringMap(monitor.ExtraHeaders)
	stored.BodyOverride = mustCloneJSONMap(monitor.BodyOverride)
	if monitor.TemplateID != nil {
		templateID := *monitor.TemplateID
		stored.TemplateID = &templateID
	}
	r.created = append(r.created, &stored)
	if stored.DuplicateOperationID != "" {
		if r.byOperation == nil {
			r.byOperation = make(map[string]*ChannelMonitor)
		}
		r.byOperation[stored.DuplicateOperationID] = &stored
	}
	return nil
}

func (r *duplicateChannelMonitorRepoStub) FindByDuplicateOperationID(_ context.Context, operationID string) (*ChannelMonitor, error) {
	monitor := r.byOperation[operationID]
	if monitor == nil {
		return nil, nil
	}
	cloned := *monitor
	cloned.ExtraModels = append([]string(nil), monitor.ExtraModels...)
	cloned.ExtraHeaders = cloneStringMap(monitor.ExtraHeaders)
	cloned.BodyOverride = mustCloneJSONMap(monitor.BodyOverride)
	return &cloned, nil
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func mustCloneJSONMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	cloned, err := cloneChannelMonitorJSONMap(source)
	if err != nil {
		panic(err)
	}
	return cloned
}

type duplicateChannelMonitorEncryptor struct {
	decryptErr error
	encryptErr error
}

func (e *duplicateChannelMonitorEncryptor) Encrypt(plaintext string) (string, error) {
	if e.encryptErr != nil {
		return "", e.encryptErr
	}
	return "NEW:" + plaintext, nil
}

func (e *duplicateChannelMonitorEncryptor) Decrypt(ciphertext string) (string, error) {
	if e.decryptErr != nil {
		return "", e.decryptErr
	}
	if !strings.HasPrefix(ciphertext, "OLD:") && !strings.HasPrefix(ciphertext, "NEW:") {
		return "", errors.New("invalid ciphertext")
	}
	return strings.TrimPrefix(strings.TrimPrefix(ciphertext, "OLD:"), "NEW:"), nil
}

func TestDuplicateChannelMonitorCopiesConfigurationAndResetsRuntimeState(t *testing.T) {
	lastCheckedAt := time.Date(2026, time.July, 15, 7, 0, 0, 0, time.UTC)
	templateID := int64(9)
	source := &ChannelMonitor{
		ID:               42,
		Name:             "primary",
		Provider:         MonitorProviderOpenAI,
		APIMode:          MonitorAPIModeResponses,
		Endpoint:         "https://api.example.com",
		APIKey:           "OLD:top-secret",
		PrimaryModel:     "gpt-5.4-mini",
		ExtraModels:      []string{"gpt-5.4", "gpt-5.3"},
		GroupName:        "production",
		Enabled:          true,
		IntervalSeconds:  90,
		JitterSeconds:    15,
		LastCheckedAt:    &lastCheckedAt,
		CreatedBy:        4,
		CreatedAt:        lastCheckedAt.Add(-time.Hour),
		UpdatedAt:        lastCheckedAt,
		TemplateID:       &templateID,
		ExtraHeaders:     map[string]string{"User-Agent": "Codex"},
		BodyOverrideMode: MonitorBodyOverrideModeMerge,
		BodyOverride: map[string]any{
			"metadata": map[string]any{"source": "original"},
		},
	}
	repo := &duplicateChannelMonitorRepoStub{source: source}
	service := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	duplicate, err := service.Duplicate(context.Background(), source.ID, 77, "admin:77", "copy-primary")

	require.NoError(t, err)
	require.Len(t, repo.created, 1)
	stored := repo.created[0]
	require.NotEqual(t, source.ID, duplicate.ID)
	require.Equal(t, "primary (Copy)", duplicate.Name)
	require.Equal(t, source.Provider, duplicate.Provider)
	require.Equal(t, source.APIMode, duplicate.APIMode)
	require.Equal(t, source.Endpoint, duplicate.Endpoint)
	require.Equal(t, "top-secret", duplicate.APIKey)
	require.Equal(t, "NEW:top-secret", stored.APIKey)
	require.Equal(t, source.PrimaryModel, duplicate.PrimaryModel)
	require.Equal(t, source.ExtraModels, duplicate.ExtraModels)
	require.Equal(t, source.GroupName, duplicate.GroupName)
	require.Equal(t, source.IntervalSeconds, duplicate.IntervalSeconds)
	require.Equal(t, source.JitterSeconds, duplicate.JitterSeconds)
	require.Equal(t, source.TemplateID, duplicate.TemplateID)
	require.Equal(t, source.ExtraHeaders, duplicate.ExtraHeaders)
	require.Equal(t, source.BodyOverrideMode, duplicate.BodyOverrideMode)
	require.Equal(t, source.BodyOverride, duplicate.BodyOverride)
	require.False(t, duplicate.Enabled)
	require.Nil(t, duplicate.LastCheckedAt)
	require.Equal(t, int64(77), duplicate.CreatedBy)
	require.False(t, duplicate.APIKeyDecryptFailed)
	require.NotEmpty(t, duplicate.DuplicateOperationID)

	duplicate.ExtraModels[0] = "changed"
	duplicate.ExtraHeaders["User-Agent"] = "changed"
	duplicate.BodyOverride["metadata"].(map[string]any)["source"] = "changed"
	*duplicate.TemplateID = 10
	require.Equal(t, []string{"gpt-5.4", "gpt-5.3"}, source.ExtraModels)
	require.Equal(t, "Codex", source.ExtraHeaders["User-Agent"])
	require.Equal(t, "original", source.BodyOverride["metadata"].(map[string]any)["source"])
	require.Equal(t, int64(9), *source.TemplateID)
	require.Equal(t, "OLD:top-secret", source.APIKey)
	require.True(t, source.Enabled)
	require.Equal(t, &lastCheckedAt, source.LastCheckedAt)
}

func TestDuplicateChannelMonitorNamePreservesSuffixWithinSchemaLimit(t *testing.T) {
	name := duplicateChannelMonitorName(strings.Repeat("界", 100))

	require.Equal(t, 100, utf8.RuneCountInString(name))
	require.True(t, strings.HasSuffix(name, " (Copy)"))
}

type customOverrideRepoStub struct {
	duplicateChannelMonitorRepoStub
	overrides map[int64]*ChannelMonitorCustomOverride
	upserts   []int64
}

func (r *customOverrideRepoStub) GetCustomOverride(_ context.Context, id int64) (*ChannelMonitorCustomOverride, error) {
	if r.overrides == nil {
		return nil, nil
	}
	return r.overrides[id], nil
}

func (r *customOverrideRepoStub) ListCustomOverrides(context.Context, []int64) (map[int64]*ChannelMonitorCustomOverride, error) {
	return map[int64]*ChannelMonitorCustomOverride{}, nil
}

func (r *customOverrideRepoStub) UpsertCustomOverride(_ context.Context, id int64, ov *ChannelMonitorCustomOverride) error {
	if r.overrides == nil {
		r.overrides = map[int64]*ChannelMonitorCustomOverride{}
	}
	r.overrides[id] = ov
	r.upserts = append(r.upserts, id)
	return nil
}

func (r *customOverrideRepoStub) DeleteCustomOverride(context.Context, int64) error { return nil }

func (r *customOverrideRepoStub) ShortestActualInterval(context.Context) (int, bool, error) {
	return 0, false, nil
}

func TestDuplicateChannelMonitorCopiesCustomOverride(t *testing.T) {
	enabled := true
	minMs, maxMs := 10, 100
	source := &ChannelMonitor{
		ID:              42,
		Name:            "primary",
		Provider:        MonitorProviderOpenAI,
		Endpoint:        "https://api.example.com",
		APIKey:          "OLD:top-secret",
		PrimaryModel:    "gpt-5.4-mini",
		IntervalSeconds: 90,
		Enabled:         true,
	}
	repo := &customOverrideRepoStub{duplicateChannelMonitorRepoStub: duplicateChannelMonitorRepoStub{source: source}}
	repo.overrides = map[int64]*ChannelMonitorCustomOverride{
		source.ID: {TTFTAdjustEnabled: &enabled, TTFTMinMs: &minMs, TTFTMaxMs: &maxMs},
	}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	duplicate, err := svc.Duplicate(context.Background(), source.ID, 77, "admin:77", "copy-override")

	require.NoError(t, err)
	require.Equal(t, []int64{duplicate.ID}, repo.upserts)
	require.Equal(t, repo.overrides[source.ID], duplicate.CustomOverride)
}

type createGuardRepo struct {
	ChannelMonitorRepository
	creates int
}

func (r *createGuardRepo) Create(context.Context, *ChannelMonitor) error {
	r.creates++
	return nil
}

func TestCreateRejectsInvalidCustomOverrideBeforeWrite(t *testing.T) {
	enabled := true
	minMs, maxMs := 20, 10
	repo := &createGuardRepo{}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	created, err := svc.Create(context.Background(), ChannelMonitorCreateParams{
		Name:            "probe",
		Provider:        MonitorProviderOpenAI,
		Endpoint:        "http://127.0.0.1:8080",
		APIKey:          "secret",
		PrimaryModel:    "gpt-5.4-mini",
		IntervalSeconds: 60,
		CustomOverride:  &ChannelMonitorCustomOverride{TTFTAdjustEnabled: &enabled, TTFTMinMs: &minMs, TTFTMaxMs: &maxMs},
	})

	require.Nil(t, created)
	require.ErrorIs(t, err, ErrChannelMonitorInvalidTTFT)
	require.Zero(t, repo.creates)
}

type overrideCatalogSettings struct {
	saved bool
}

func (s *overrideCatalogSettings) GetChannelMonitorRuntime(context.Context) ChannelMonitorRuntime {
	return ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeCustom}
}

func (s *overrideCatalogSettings) GetChannelMonitorCustom(context.Context) (ChannelMonitorCustomConfig, bool) {
	cfg := defaultChannelMonitorCustomConfig()
	cfg.TTFTAdjustEnabled = true
	cfg.TTFTMinMs = 100
	cfg.TTFTMaxMs = 1000
	return cfg, true
}

func (s *overrideCatalogSettings) SaveChannelMonitorMode(context.Context, string, ChannelMonitorCustomConfig) error {
	s.saved = true
	return nil
}

type overrideCatalogRepo struct {
	ChannelMonitorRepository
	items []*ChannelMonitorCustomOverride
}

func (r *overrideCatalogRepo) ListAllCustomOverrides(context.Context) ([]*ChannelMonitorCustomOverride, error) {
	return r.items, nil
}

func (r *overrideCatalogRepo) ShortestActualInterval(context.Context) (int, bool, error) {
	return 0, false, nil
}

func TestSaveMonitorModeRejectsBrokenExistingOverride(t *testing.T) {
	maxMs := 50
	repo := &overrideCatalogRepo{items: []*ChannelMonitorCustomOverride{{TTFTMaxMs: &maxMs}}}
	settings := &overrideCatalogSettings{}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	svc.SetRuntimeReader(settings)

	cfg := defaultChannelMonitorCustomConfig()
	cfg.TTFTAdjustEnabled = true
	cfg.TTFTMinMs = 100
	cfg.TTFTMaxMs = 1000
	err := svc.SaveMonitorMode(context.Background(), ChannelMonitorModeCustom, cfg)

	require.ErrorIs(t, err, ErrChannelMonitorInvalidTTFT)
	require.False(t, settings.saved)
}

func TestDuplicateChannelMonitorRejectsUndecryptableAPIKey(t *testing.T) {
	source := &ChannelMonitor{ID: 42, Name: "broken", APIKey: "OLD:broken"}
	repo := &duplicateChannelMonitorRepoStub{source: source}
	service := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{decryptErr: errors.New("wrong encryption key")})

	duplicate, err := service.Duplicate(context.Background(), source.ID, 77, "admin:77", "copy-broken")

	require.Nil(t, duplicate)
	require.ErrorIs(t, err, ErrChannelMonitorAPIKeyDecryptFailed)
	require.Empty(t, repo.created)
	require.Equal(t, "OLD:broken", source.APIKey)
}

func TestDuplicateChannelMonitorRecoversCommittedCopyForSameOperation(t *testing.T) {
	source := &ChannelMonitor{
		ID:               42,
		Name:             "primary",
		Provider:         MonitorProviderOpenAI,
		APIMode:          MonitorAPIModeResponses,
		Endpoint:         "https://api.example.com",
		APIKey:           "OLD:top-secret",
		PrimaryModel:     "gpt-5.4-mini",
		IntervalSeconds:  60,
		BodyOverrideMode: MonitorBodyOverrideModeOff,
	}
	repo := &duplicateChannelMonitorRepoStub{source: source}
	service := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	first, err := service.Duplicate(context.Background(), source.ID, 77, "admin:77", "stable-key")
	require.NoError(t, err)
	retry, err := service.Duplicate(context.Background(), source.ID, 77, "admin:77", "stable-key")
	require.NoError(t, err)

	require.Len(t, repo.created, 1, "same operation must not create a second monitor")
	require.Equal(t, first.ID, retry.ID)
	require.Equal(t, "top-secret", retry.APIKey)
	require.Equal(t, first.DuplicateOperationID, retry.DuplicateOperationID)
	require.NotContains(t, retry.ExtraHeaders, ChannelMonitorDuplicateOperationIDMetadataKey)

	otherActor, err := service.Duplicate(context.Background(), source.ID, 88, "admin:88", "stable-key")
	require.NoError(t, err)
	require.NotEqual(t, first.ID, otherActor.ID)
	require.Len(t, repo.created, 2, "operation identity must include the actor scope")
}

func TestChannelMonitorDuplicateOperationMetadataKeyCannotBeSubmittedAsHeader(t *testing.T) {
	err := validateExtraHeaders(map[string]string{
		ChannelMonitorDuplicateOperationIDMetadataKey: "forged-operation",
	})

	require.Error(t, err)
}
