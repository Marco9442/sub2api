//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSortUserMonitorViewsKeepsCatalogThenOrphansThenUngrouped(t *testing.T) {
	groups := []ChannelMonitorDisplayGroup{
		{ID: 2, Name: "beta", SortOrder: 0},
		{ID: 1, Name: "alpha", SortOrder: 1},
	}
	views := []*UserMonitorView{
		{ID: 9, GroupName: "", DisplaySort: 0},
		{ID: 4, GroupName: "orphan-b", DisplaySort: 1},
		{ID: 3, GroupName: "alpha", DisplaySort: 2},
		{ID: 8, GroupName: "orphan-a", DisplaySort: 0},
		{ID: 1, GroupName: "beta", DisplaySort: 1},
		{ID: 2, GroupName: "beta", DisplaySort: 0},
		{ID: 7, GroupName: "alpha", DisplaySort: 0},
		{ID: 5, GroupName: "orphan-b", DisplaySort: 0},
	}

	sortUserMonitorViews(views, groups)

	got := make([]int64, len(views))
	for i, view := range views {
		got[i] = view.ID
	}
	require.Equal(t, []int64{2, 1, 7, 3, 5, 4, 8, 9}, got)
}

func TestValidateDisplayLayoutRejectsIncompleteSets(t *testing.T) {
	groups := []ChannelMonitorDisplayGroup{{ID: 1, Name: "alpha"}, {ID: 2, Name: "beta"}}
	members := []ChannelMonitorDisplayMembership{{ID: 10}, {ID: 11}}

	err := validateDisplayLayout(ChannelMonitorDisplayLayout{
		Groups:       []ChannelMonitorDisplayLayoutGroup{{ID: 1, MonitorIDs: []int64{10, 11}}},
		UngroupedIDs: nil,
	}, groups, members)
	require.ErrorIs(t, err, ErrChannelMonitorDisplayLayoutInvalid)

	err = validateDisplayLayout(ChannelMonitorDisplayLayout{
		Groups: []ChannelMonitorDisplayLayoutGroup{
			{ID: 1, MonitorIDs: []int64{10}},
			{ID: 2, MonitorIDs: []int64{10}},
		},
	}, groups, members)
	require.ErrorIs(t, err, ErrChannelMonitorDisplayLayoutInvalid)

	err = validateDisplayLayout(ChannelMonitorDisplayLayout{
		Groups: []ChannelMonitorDisplayLayoutGroup{
			{ID: 1, MonitorIDs: []int64{10}},
			{ID: 99, MonitorIDs: []int64{11}},
		},
	}, groups, members)
	require.ErrorIs(t, err, ErrChannelMonitorDisplayLayoutInvalid)
}

func TestAssembleDisplayAdminViewLeavesOrphansOutOfUngrouped(t *testing.T) {
	view := assembleDisplayAdminView(
		[]ChannelMonitorDisplayGroup{{ID: 1, Name: "alpha", SortOrder: 0}},
		[]ChannelMonitorDisplayMembership{
			{ID: 2, GroupName: "alpha"},
			{ID: 3, GroupName: ""},
			{ID: 4, GroupName: "legacy"},
		},
	)

	require.Equal(t, []int64{2}, view.Groups[0].MonitorIDs)
	require.Equal(t, []int64{3}, view.UngroupedIDs)
	require.Equal(t, "legacy", view.Monitors[2].GroupName)
}

func TestRejectDisplayGroupEditOnlyBlocksADifferentName(t *testing.T) {
	same := "alpha"
	other := "beta"
	require.NoError(t, rejectDisplayGroupEdit("alpha", nil))
	require.NoError(t, rejectDisplayGroupEdit("alpha", &same))
	require.ErrorIs(t, rejectDisplayGroupEdit("alpha", &other), ErrChannelMonitorDisplayGroupLayoutRequired)
}

type displayGroupRepoStub struct {
	ChannelMonitorRepository
	groups     []ChannelMonitorDisplayGroup
	enabled    []*ChannelMonitor
	nextSort   int
	nextCalls  int
	creates    int
	updates    int
	listGroupN int
}

func (r *displayGroupRepoStub) ListDisplayGroups(context.Context) ([]ChannelMonitorDisplayGroup, error) {
	r.listGroupN++
	return append([]ChannelMonitorDisplayGroup(nil), r.groups...), nil
}

func (r *displayGroupRepoStub) ListEnabled(context.Context) ([]*ChannelMonitor, error) {
	return r.enabled, nil
}

func (r *displayGroupRepoStub) NextDisplaySort(context.Context, string) (int, error) {
	r.nextCalls++
	return r.nextSort, nil
}

func (r *displayGroupRepoStub) Create(_ context.Context, monitor *ChannelMonitor) error {
	r.creates++
	monitor.ID = 41
	return nil
}

func (r *displayGroupRepoStub) GetByID(context.Context, int64) (*ChannelMonitor, error) {
	return &ChannelMonitor{ID: 7, GroupName: "alpha", Provider: MonitorProviderOpenAI, Endpoint: "http://127.0.0.1:8080", IntervalSeconds: 60, PrimaryModel: "gpt-5.4-mini"}, nil
}

func (r *displayGroupRepoStub) Update(context.Context, *ChannelMonitor) error {
	r.updates++
	return nil
}

func TestCreateUnknownDisplayGroupWritesNothing(t *testing.T) {
	repo := &displayGroupRepoStub{groups: []ChannelMonitorDisplayGroup{{ID: 1, Name: "alpha"}}}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	created, err := svc.Create(context.Background(), ChannelMonitorCreateParams{
		Name:            "probe",
		Provider:        MonitorProviderOpenAI,
		Endpoint:        "http://127.0.0.1:8080",
		APIKey:          "secret",
		PrimaryModel:    "gpt-5.4-mini",
		GroupName:       "missing",
		IntervalSeconds: 60,
	})

	require.Nil(t, created)
	require.ErrorIs(t, err, ErrChannelMonitorDisplayGroupInvalid)
	require.Zero(t, repo.nextCalls)
	require.Zero(t, repo.creates)
}

func TestCreateAssignsTailInsideKnownGroup(t *testing.T) {
	repo := &displayGroupRepoStub{
		groups:   []ChannelMonitorDisplayGroup{{ID: 1, Name: "alpha"}},
		nextSort: 4,
	}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	created, err := svc.Create(context.Background(), ChannelMonitorCreateParams{
		Name:            "probe",
		Provider:        MonitorProviderOpenAI,
		Endpoint:        "http://127.0.0.1:8080",
		APIKey:          "secret",
		PrimaryModel:    "gpt-5.4-mini",
		GroupName:       " alpha ",
		IntervalSeconds: 60,
	})

	require.NoError(t, err)
	require.Equal(t, 4, created.DisplaySort)
	require.Equal(t, "alpha", created.GroupName)
	require.Equal(t, 1, repo.creates)
}

func TestUpdateDifferentDisplayGroupWritesNothing(t *testing.T) {
	repo := &displayGroupRepoStub{}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	next := "beta"

	updated, err := svc.Update(context.Background(), 7, ChannelMonitorUpdateParams{GroupName: &next})

	require.Nil(t, updated)
	require.ErrorIs(t, err, ErrChannelMonitorDisplayGroupLayoutRequired)
	require.Zero(t, repo.updates)
}

func TestListUserViewLoadsCatalogBeforeEmptyShortCircuit(t *testing.T) {
	repo := &displayGroupRepoStub{groups: []ChannelMonitorDisplayGroup{{ID: 3, Name: "alpha", SortOrder: 0}}}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	list, err := svc.ListUserView(context.Background(), "")

	require.NoError(t, err)
	require.Empty(t, list.Items)
	require.Equal(t, repo.groups, list.Groups)
	require.Equal(t, 1, repo.listGroupN)
}

type layoutTxRepo struct {
	displayGroupRepoStub
	members []ChannelMonitorDisplayMembership
	applied int
}

func (r *layoutTxRepo) WithMonitorTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (r *layoutTxRepo) ListDisplayMemberships(context.Context) ([]ChannelMonitorDisplayMembership, error) {
	return append([]ChannelMonitorDisplayMembership(nil), r.members...), nil
}

func (r *layoutTxRepo) ApplyDisplayLayout(context.Context, ChannelMonitorDisplayLayoutPlan) error {
	r.applied++
	return nil
}

func TestApplyDisplayLayoutRejectsIncompleteSetBeforeWrite(t *testing.T) {
	repo := &layoutTxRepo{
		displayGroupRepoStub: displayGroupRepoStub{groups: []ChannelMonitorDisplayGroup{{ID: 1, Name: "alpha"}}},
		members:              []ChannelMonitorDisplayMembership{{ID: 10, GroupName: "alpha"}},
	}
	svc := NewChannelMonitorService(repo, nil)

	err := svc.ApplyDisplayLayout(context.Background(), ChannelMonitorDisplayLayout{
		UngroupedIDs: []int64{10},
	})

	require.ErrorIs(t, err, ErrChannelMonitorDisplayLayoutInvalid)
	require.Zero(t, repo.applied)
}

func TestDisplayGroupWritesRequireMonitorTx(t *testing.T) {
	repo := &displayGroupRepoStub{}
	svc := NewChannelMonitorService(repo, nil)

	require.Error(t, svc.RenameDisplayGroup(context.Background(), 1, "beta"))
	require.Error(t, svc.DeleteDisplayGroup(context.Background(), 1))
	require.Error(t, svc.ApplyDisplayLayout(context.Background(), ChannelMonitorDisplayLayout{}))
}

type rollbackDisplayRepo struct {
	displayGroupRepoStub
	committed  bool
	rolledBack bool
}

func (r *rollbackDisplayRepo) WithMonitorTx(ctx context.Context, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		r.rolledBack = true
		return err
	}
	r.committed = true
	return nil
}

func (r *rollbackDisplayRepo) RenameDisplayGroup(context.Context, int64, string) error {
	return errors.New("member update failed")
}

func (r *rollbackDisplayRepo) DeleteDisplayGroup(context.Context, int64) (int64, error) {
	return 0, errors.New("member update failed")
}

func TestDisplayGroupMutationRollsBackWhenMemberUpdateFails(t *testing.T) {
	repo := &rollbackDisplayRepo{}
	svc := NewChannelMonitorService(repo, nil)

	err := svc.RenameDisplayGroup(context.Background(), 1, "beta")
	require.EqualError(t, err, "member update failed")
	require.True(t, repo.rolledBack)
	require.False(t, repo.committed)

	repo.rolledBack = false
	err = svc.DeleteDisplayGroup(context.Background(), 1)
	require.EqualError(t, err, "member update failed")
	require.True(t, repo.rolledBack)
	require.False(t, repo.committed)
}
