package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type channelMonitorCustomOverrideRequest struct {
	TTFTAdjustEnabled *bool    `json:"ttft_adjust_enabled"`
	TTFTMultiplier    *float64 `json:"ttft_multiplier"`
	TTFTMinMs         *int     `json:"ttft_min_ms"`
	TTFTMaxMs         *int     `json:"ttft_max_ms"`
	TPSEnabled        *bool    `json:"tps_enabled"`
	TPSMin            *float64 `json:"tps_min"`
	TPSMax            *float64 `json:"tps_max"`
}

type channelMonitorCustomConfigRequest struct {
	Prompt            string  `json:"prompt"`
	YellowSeconds     int     `json:"yellow_seconds"`
	RedSeconds        int     `json:"red_seconds"`
	TTFTAdjustEnabled bool    `json:"ttft_adjust_enabled"`
	TTFTMultiplier    float64 `json:"ttft_multiplier"`
	TTFTMinMs         int     `json:"ttft_min_ms"`
	TTFTMaxMs         int     `json:"ttft_max_ms"`
	TPSEnabled        bool    `json:"tps_enabled"`
	TPSMin            float64 `json:"tps_min"`
	TPSMax            float64 `json:"tps_max"`
}

type channelMonitorModeRequest struct {
	Mode   string                            `json:"mode" binding:"required,oneof=v1 v2 custom"`
	Custom channelMonitorCustomConfigRequest `json:"custom"`
}

func customOverrideFromRequest(req *channelMonitorCustomOverrideRequest) *service.ChannelMonitorCustomOverride {
	if req == nil {
		return nil
	}
	return &service.ChannelMonitorCustomOverride{
		TTFTAdjustEnabled: req.TTFTAdjustEnabled,
		TTFTMultiplier:    req.TTFTMultiplier,
		TTFTMinMs:         req.TTFTMinMs,
		TTFTMaxMs:         req.TTFTMaxMs,
		TPSEnabled:        req.TPSEnabled,
		TPSMin:            req.TPSMin,
		TPSMax:            req.TPSMax,
	}
}

func customConfigFromRequest(req channelMonitorCustomConfigRequest) service.ChannelMonitorCustomConfig {
	prompt := req.Prompt
	yellow, red := req.YellowSeconds, req.RedSeconds
	if prompt == "" && yellow == 0 && red == 0 {
		prompt = "ping"
		yellow, red = 2, 8
	}
	multiplier := req.TTFTMultiplier
	if multiplier == 0 {
		multiplier = 1
	}
	return service.ChannelMonitorCustomConfig{
		Prompt:            prompt,
		YellowSeconds:     yellow,
		RedSeconds:        red,
		TTFTAdjustEnabled: req.TTFTAdjustEnabled,
		TTFTMultiplier:    multiplier,
		TTFTMinMs:         req.TTFTMinMs,
		TTFTMaxMs:         req.TTFTMaxMs,
		TPSEnabled:        req.TPSEnabled,
		TPSMin:            req.TPSMin,
		TPSMax:            req.TPSMax,
	}
}

// GetMode GET /api/v1/admin/channel-monitors/mode
func (h *ChannelMonitorHandler) GetMode(c *gin.Context) {
	view := h.monitorService.GetMonitorMode(c.Request.Context())
	response.Success(c, gin.H{
		"mode":                 view.Mode,
		"enabled":              view.Enabled,
		"thresholds_persisted": view.ThresholdsPersisted,
		"custom":               view.Custom,
	})
}

// UpdateMode PUT /api/v1/admin/channel-monitors/mode
func (h *ChannelMonitorHandler) UpdateMode(c *gin.Context) {
	var req channelMonitorModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.monitorService.SaveMonitorMode(c.Request.Context(), req.Mode, customConfigFromRequest(req.Custom)); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.GetMode(c)
}
