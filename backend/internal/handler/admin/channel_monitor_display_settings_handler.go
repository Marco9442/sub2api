package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type channelMonitorDisplaySettingsRequest struct {
	DefaultIntervalSeconds int `json:"default_interval_seconds"`
	TimelinePoints         int `json:"timeline_points"`
}

// GetDisplaySettings GET /api/v1/admin/channel-monitors/display-settings
func (h *ChannelMonitorHandler) GetDisplaySettings(c *gin.Context) {
	response.Success(c, h.monitorService.DisplaySettings(c.Request.Context()))
}

// UpdateDisplaySettings PUT /api/v1/admin/channel-monitors/display-settings
func (h *ChannelMonitorHandler) UpdateDisplaySettings(c *gin.Context) {
	var req channelMonitorDisplaySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, service.ErrChannelMonitorInvalidInterval)
		return
	}
	if err := h.monitorService.SaveDisplaySettings(c.Request.Context(), req.DefaultIntervalSeconds, req.TimelinePoints); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, h.monitorService.DisplaySettings(c.Request.Context()))
}
