package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type displayGroupResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type displayGroupViewResponse struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	SortOrder  int     `json:"sort_order"`
	MonitorIDs []int64 `json:"monitor_ids"`
}

type displayMembershipResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Enabled     bool   `json:"enabled"`
	GroupName   string `json:"group_name"`
	DisplaySort int    `json:"display_sort"`
}

type displayAdminResponse struct {
	Groups       []displayGroupViewResponse  `json:"groups"`
	UngroupedIDs []int64                     `json:"ungrouped_ids"`
	Monitors     []displayMembershipResponse `json:"monitors"`
}

type displayGroupNameRequest struct {
	Name string `json:"name"`
}

type displayLayoutRequest struct {
	Groups []struct {
		ID         int64   `json:"id"`
		MonitorIDs []int64 `json:"monitor_ids"`
	} `json:"groups"`
	UngroupedIDs []int64 `json:"ungrouped_ids"`
}

func parseDisplayGroupID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, service.ErrChannelMonitorDisplayGroupInvalid)
		return 0, false
	}
	return id, true
}

func displayGroupInvalid(c *gin.Context) {
	response.ErrorFrom(c, service.ErrChannelMonitorDisplayGroupInvalid)
}

// ListDisplayGroups GET /api/v1/admin/channel-monitors/display-groups
func (h *ChannelMonitorHandler) ListDisplayGroups(c *gin.Context) {
	view, err := h.monitorService.ListDisplayGroupsAdmin(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, displayAdminToResponse(view))
}

// CreateDisplayGroup POST /api/v1/admin/channel-monitors/display-groups
func (h *ChannelMonitorHandler) CreateDisplayGroup(c *gin.Context) {
	var req displayGroupNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		displayGroupInvalid(c)
		return
	}
	group, err := h.monitorService.CreateDisplayGroup(c.Request.Context(), req.Name)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, displayGroupResponse{ID: group.ID, Name: group.Name, SortOrder: group.SortOrder})
}

// RenameDisplayGroup PUT /api/v1/admin/channel-monitors/display-groups/:id
func (h *ChannelMonitorHandler) RenameDisplayGroup(c *gin.Context) {
	id, ok := parseDisplayGroupID(c)
	if !ok {
		return
	}
	var req displayGroupNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		displayGroupInvalid(c)
		return
	}
	if err := h.monitorService.RenameDisplayGroup(c.Request.Context(), id, req.Name); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// DeleteDisplayGroup DELETE /api/v1/admin/channel-monitors/display-groups/:id
func (h *ChannelMonitorHandler) DeleteDisplayGroup(c *gin.Context) {
	id, ok := parseDisplayGroupID(c)
	if !ok {
		return
	}
	if err := h.monitorService.DeleteDisplayGroup(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// ApplyDisplayLayout PUT /api/v1/admin/channel-monitors/display-groups/layout
func (h *ChannelMonitorHandler) ApplyDisplayLayout(c *gin.Context) {
	var req displayLayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		displayGroupInvalid(c)
		return
	}
	layout := service.ChannelMonitorDisplayLayout{
		Groups:       make([]service.ChannelMonitorDisplayLayoutGroup, 0, len(req.Groups)),
		UngroupedIDs: append([]int64(nil), req.UngroupedIDs...),
	}
	for _, group := range req.Groups {
		layout.Groups = append(layout.Groups, service.ChannelMonitorDisplayLayoutGroup{
			ID:         group.ID,
			MonitorIDs: append([]int64(nil), group.MonitorIDs...),
		})
	}
	if err := h.monitorService.ApplyDisplayLayout(c.Request.Context(), layout); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

func displayAdminToResponse(view *service.ChannelMonitorDisplayAdminView) displayAdminResponse {
	if view == nil {
		return displayAdminResponse{
			Groups:       []displayGroupViewResponse{},
			UngroupedIDs: []int64{},
			Monitors:     []displayMembershipResponse{},
		}
	}
	groups := make([]displayGroupViewResponse, 0, len(view.Groups))
	for _, group := range view.Groups {
		ids := group.MonitorIDs
		if ids == nil {
			ids = []int64{}
		}
		groups = append(groups, displayGroupViewResponse{
			ID:         group.ID,
			Name:       group.Name,
			SortOrder:  group.SortOrder,
			MonitorIDs: ids,
		})
	}
	ungrouped := view.UngroupedIDs
	if ungrouped == nil {
		ungrouped = []int64{}
	}
	monitors := make([]displayMembershipResponse, 0, len(view.Monitors))
	for _, monitor := range view.Monitors {
		monitors = append(monitors, displayMembershipResponse{
			ID:          monitor.ID,
			Name:        monitor.Name,
			Provider:    monitor.Provider,
			Enabled:     monitor.Enabled,
			GroupName:   monitor.GroupName,
			DisplaySort: monitor.DisplaySort,
		})
	}
	return displayAdminResponse{Groups: groups, UngroupedIDs: ungrouped, Monitors: monitors}
}
