package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"unicode/utf8"
)

// MaxChannelMonitorDisplayGroups is the catalog cap. Empty groups count.
const MaxChannelMonitorDisplayGroups = 100

// ChannelMonitorDisplayGroup is one section in the status-page catalog.
// Membership stays on channel_monitors.group_name.
type ChannelMonitorDisplayGroup struct {
	ID        int64
	Name      string
	SortOrder int
}

// ChannelMonitorDisplayMembership is the thin monitor row used by the admin panel.
// It never carries endpoint, API key, headers, body, or account id.
type ChannelMonitorDisplayMembership struct {
	ID          int64
	Name        string
	Provider    string
	Enabled     bool
	GroupName   string
	DisplaySort int
}

// ChannelMonitorDisplayLayoutGroup is one catalog row in a layout write.
type ChannelMonitorDisplayLayoutGroup struct {
	ID         int64
	MonitorIDs []int64
}

// ChannelMonitorDisplayLayout is the admin save payload.
// Group order is the new sort_order. Monitor order inside a group is display_sort.
type ChannelMonitorDisplayLayout struct {
	Groups       []ChannelMonitorDisplayLayoutGroup
	UngroupedIDs []int64
}

// ChannelMonitorDisplayLayoutWrite is a validated layout ready for one transaction.
type ChannelMonitorDisplayLayoutWrite struct {
	GroupID    int64
	SortOrder  int
	Name       string
	MonitorIDs []int64
}

// ChannelMonitorDisplayLayoutPlan is the full write set for one layout save.
type ChannelMonitorDisplayLayoutPlan struct {
	Groups       []ChannelMonitorDisplayLayoutWrite
	UngroupedIDs []int64
}

// ChannelMonitorDisplayGroupView is one catalog row plus its member ids.
type ChannelMonitorDisplayGroupView struct {
	ID         int64
	Name       string
	SortOrder  int
	MonitorIDs []int64
}

// ChannelMonitorDisplayAdminView is the dedicated admin read model.
type ChannelMonitorDisplayAdminView struct {
	Groups       []ChannelMonitorDisplayGroupView
	UngroupedIDs []int64
	Monitors     []ChannelMonitorDisplayMembership
}

// UserMonitorList keeps the catalog when no monitor is enabled.
type UserMonitorList struct {
	Items  []*UserMonitorView
	Groups []ChannelMonitorDisplayGroup
}

func normalizeDisplayGroupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxChannelMonitorNameRunes {
		return "", ErrChannelMonitorDisplayGroupInvalid
	}
	return name, nil
}

func rejectDisplayGroupEdit(current string, next *string) error {
	if next == nil {
		return nil
	}
	if strings.TrimSpace(*next) == current {
		return nil
	}
	return ErrChannelMonitorDisplayGroupLayoutRequired
}

func (s *ChannelMonitorService) withDisplayGroupTx(ctx context.Context, fn func(context.Context) error) error {
	runner, ok := s.repo.(channelMonitorTxRunner)
	if !ok {
		return fmt.Errorf("channel monitor display group tx runner is not configured")
	}
	return runner.WithMonitorTx(ctx, fn)
}

func (s *ChannelMonitorService) ensureDisplayGroup(ctx context.Context, name string) error {
	if name == "" {
		return nil
	}
	groups, err := s.repo.ListDisplayGroups(ctx)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group.Name == name {
			return nil
		}
	}
	return ErrChannelMonitorDisplayGroupInvalid
}

func (s *ChannelMonitorService) assignDisplaySort(ctx context.Context, monitor *ChannelMonitor) error {
	if monitor == nil {
		return nil
	}
	next, err := s.repo.NextDisplaySort(ctx, monitor.GroupName)
	if err != nil {
		return err
	}
	monitor.DisplaySort = next
	return nil
}

// ListDisplayGroupsAdmin returns every monitor without decrypting secrets.
func (s *ChannelMonitorService) ListDisplayGroupsAdmin(ctx context.Context) (*ChannelMonitorDisplayAdminView, error) {
	groups, err := s.repo.ListDisplayGroups(ctx)
	if err != nil {
		return nil, err
	}
	members, err := s.repo.ListDisplayMemberships(ctx)
	if err != nil {
		return nil, err
	}
	return assembleDisplayAdminView(groups, members), nil
}

// CreateDisplayGroup appends a catalog row. sort_order is the current max + 1.
func (s *ChannelMonitorService) CreateDisplayGroup(ctx context.Context, name string) (*ChannelMonitorDisplayGroup, error) {
	name, err := normalizeDisplayGroupName(name)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateDisplayGroup(ctx, name)
}

// RenameDisplayGroup updates the catalog name and every member in one transaction.
func (s *ChannelMonitorService) RenameDisplayGroup(ctx context.Context, id int64, name string) error {
	name, err := normalizeDisplayGroupName(name)
	if err != nil {
		return err
	}
	return s.withDisplayGroupTx(ctx, func(txCtx context.Context) error {
		return s.repo.RenameDisplayGroup(txCtx, id, name)
	})
}

// DeleteDisplayGroup clears member names, then removes the catalog row.
// Monitors and history stay. A missing id writes nothing.
func (s *ChannelMonitorService) DeleteDisplayGroup(ctx context.Context, id int64) error {
	var affected int64
	err := s.withDisplayGroupTx(ctx, func(txCtx context.Context) error {
		n, err := s.repo.DeleteDisplayGroup(txCtx, id)
		affected = n
		return err
	})
	if err != nil {
		return err
	}
	slog.Info("channel_monitor_display_group_deleted", "display_group_id", id, "affected_monitors", affected)
	return nil
}

// ApplyDisplayLayout rejects an incomplete id set before any write.
func (s *ChannelMonitorService) ApplyDisplayLayout(ctx context.Context, layout ChannelMonitorDisplayLayout) error {
	return s.withDisplayGroupTx(ctx, func(txCtx context.Context) error {
		groups, err := s.repo.ListDisplayGroups(txCtx)
		if err != nil {
			return err
		}
		members, err := s.repo.ListDisplayMemberships(txCtx)
		if err != nil {
			return err
		}
		plan, err := buildDisplayLayoutPlan(layout, groups, members)
		if err != nil {
			return err
		}
		return s.repo.ApplyDisplayLayout(txCtx, plan)
	})
}

func assembleDisplayAdminView(groups []ChannelMonitorDisplayGroup, members []ChannelMonitorDisplayMembership) *ChannelMonitorDisplayAdminView {
	byName := make(map[string][]int64, len(groups))
	ungrouped := make([]int64, 0)
	monitors := make([]ChannelMonitorDisplayMembership, 0, len(members))
	for _, member := range members {
		monitors = append(monitors, member)
		if member.GroupName == "" {
			ungrouped = append(ungrouped, member.ID)
			continue
		}
		byName[member.GroupName] = append(byName[member.GroupName], member.ID)
	}
	views := make([]ChannelMonitorDisplayGroupView, 0, len(groups))
	for _, group := range groups {
		ids := byName[group.Name]
		if ids == nil {
			ids = []int64{}
		}
		views = append(views, ChannelMonitorDisplayGroupView{
			ID:         group.ID,
			Name:       group.Name,
			SortOrder:  group.SortOrder,
			MonitorIDs: ids,
		})
	}
	return &ChannelMonitorDisplayAdminView{
		Groups:       views,
		UngroupedIDs: ungrouped,
		Monitors:     monitors,
	}
}

func buildDisplayLayoutPlan(layout ChannelMonitorDisplayLayout, groups []ChannelMonitorDisplayGroup, members []ChannelMonitorDisplayMembership) (ChannelMonitorDisplayLayoutPlan, error) {
	if err := validateDisplayLayout(layout, groups, members); err != nil {
		return ChannelMonitorDisplayLayoutPlan{}, err
	}
	names := make(map[int64]string, len(groups))
	for _, group := range groups {
		names[group.ID] = group.Name
	}
	writes := make([]ChannelMonitorDisplayLayoutWrite, 0, len(layout.Groups))
	for i, group := range layout.Groups {
		writes = append(writes, ChannelMonitorDisplayLayoutWrite{
			GroupID:    group.ID,
			SortOrder:  i,
			Name:       names[group.ID],
			MonitorIDs: append([]int64(nil), group.MonitorIDs...),
		})
	}
	return ChannelMonitorDisplayLayoutPlan{
		Groups:       writes,
		UngroupedIDs: append([]int64(nil), layout.UngroupedIDs...),
	}, nil
}

func validateDisplayLayout(layout ChannelMonitorDisplayLayout, groups []ChannelMonitorDisplayGroup, members []ChannelMonitorDisplayMembership) error {
	wantGroups := make(map[int64]struct{}, len(groups))
	for _, group := range groups {
		if _, seen := wantGroups[group.ID]; seen {
			return ErrChannelMonitorDisplayLayoutInvalid
		}
		wantGroups[group.ID] = struct{}{}
	}
	if len(layout.Groups) != len(wantGroups) {
		return ErrChannelMonitorDisplayLayoutInvalid
	}
	for _, group := range layout.Groups {
		if _, ok := wantGroups[group.ID]; !ok {
			return ErrChannelMonitorDisplayLayoutInvalid
		}
		delete(wantGroups, group.ID)
	}
	if len(wantGroups) != 0 {
		return ErrChannelMonitorDisplayLayoutInvalid
	}

	wantMonitors := make(map[int64]struct{}, len(members))
	for _, member := range members {
		wantMonitors[member.ID] = struct{}{}
	}
	seen := make(map[int64]struct{}, len(members))
	check := func(ids []int64) error {
		for _, id := range ids {
			if _, ok := wantMonitors[id]; !ok {
				return ErrChannelMonitorDisplayLayoutInvalid
			}
			if _, dup := seen[id]; dup {
				return ErrChannelMonitorDisplayLayoutInvalid
			}
			seen[id] = struct{}{}
		}
		return nil
	}
	for _, group := range layout.Groups {
		if err := check(group.MonitorIDs); err != nil {
			return err
		}
	}
	if err := check(layout.UngroupedIDs); err != nil {
		return err
	}
	if len(seen) != len(wantMonitors) {
		return ErrChannelMonitorDisplayLayoutInvalid
	}
	return nil
}

// sortUserMonitorViews orders an already built user list.
// Catalog names come first by sort_order, unknown names next by their smallest id,
// and the empty name last. Ties break by display_sort, then id.
func sortUserMonitorViews(views []*UserMonitorView, groups []ChannelMonitorDisplayGroup) {
	if len(views) < 2 {
		return
	}
	order := make(map[string]int, len(groups))
	for _, group := range groups {
		if _, ok := order[group.Name]; !ok {
			order[group.Name] = group.SortOrder
		}
	}
	minID := make(map[string]int64)
	for _, view := range views {
		if view == nil || view.GroupName == "" {
			continue
		}
		if _, ok := order[view.GroupName]; ok {
			continue
		}
		if cur, ok := minID[view.GroupName]; !ok || view.ID < cur {
			minID[view.GroupName] = view.ID
		}
	}
	sort.SliceStable(views, func(i, j int) bool {
		return displayViewLess(views[i], views[j], order, minID)
	})
}

func displayViewLess(a, b *UserMonitorView, order map[string]int, minID map[string]int64) bool {
	ar, ao, am := displayViewRank(a, order, minID)
	br, bo, bm := displayViewRank(b, order, minID)
	if ar != br {
		return ar < br
	}
	if ar == 0 && ao != bo {
		return ao < bo
	}
	if ar == 1 && am != bm {
		return am < bm
	}
	if ar == 1 && a.GroupName != b.GroupName {
		return a.GroupName < b.GroupName
	}
	if a.DisplaySort != b.DisplaySort {
		return a.DisplaySort < b.DisplaySort
	}
	return a.ID < b.ID
}

func displayViewRank(view *UserMonitorView, order map[string]int, minID map[string]int64) (rank, sortOrder int, min int64) {
	if view == nil || view.GroupName == "" {
		return 2, 0, 0
	}
	if sortOrder, ok := order[view.GroupName]; ok {
		return 0, sortOrder, 0
	}
	return 1, 0, minID[view.GroupName]
}
