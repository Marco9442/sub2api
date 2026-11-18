//go:build unit

package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type displayRouteRepo struct {
	service.ChannelMonitorRepository
}

func (displayRouteRepo) ListDisplayGroups(context.Context) ([]service.ChannelMonitorDisplayGroup, error) {
	return []service.ChannelMonitorDisplayGroup{{ID: 3, Name: "openai", SortOrder: 0}}, nil
}

func (displayRouteRepo) ListDisplayMemberships(context.Context) ([]service.ChannelMonitorDisplayMembership, error) {
	return []service.ChannelMonitorDisplayMembership{{
		ID: 9, Name: "gpt", Provider: "openai", Enabled: true, GroupName: "openai", DisplaySort: 0,
	}}, nil
}

func TestDisplayGroupRoutesAreNotCapturedByMonitorID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	monitorService := service.NewChannelMonitorService(displayRouteRepo{}, nil)
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		ChannelMonitor: admin.NewChannelMonitorHandler(monitorService),
	}}
	router := gin.New()
	registerChannelMonitorRoutes(router.Group("/api/v1/admin"), handlers, &service.SettingService{})

	var layoutPath string
	for _, route := range router.Routes() {
		if route.Method == http.MethodPut && route.Path == "/api/v1/admin/channel-monitors/display-groups/layout" {
			layoutPath = route.Path
		}
	}
	require.NotEmpty(t, layoutPath)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/channel-monitors/display-groups", nil))
	require.Equal(t, http.StatusOK, response.Code)

	var body struct {
		Reason string `json:"reason"`
		Data   struct {
			Groups []struct {
				Name       string  `json:"name"`
				MonitorIDs []int64 `json:"monitor_ids"`
			} `json:"groups"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Empty(t, body.Reason)
	require.Equal(t, "openai", body.Data.Groups[0].Name)
	require.Equal(t, []int64{9}, body.Data.Groups[0].MonitorIDs)

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/channel-monitors/display-settings", nil))
	require.Equal(t, http.StatusOK, response.Code)
	var settingsBody struct {
		Reason string `json:"reason"`
		Data   struct {
			DefaultIntervalSeconds int `json:"default_interval_seconds"`
			TimelinePoints         int `json:"timeline_points"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &settingsBody))
	require.Empty(t, settingsBody.Reason)
	require.Equal(t, 300, settingsBody.Data.DefaultIntervalSeconds)
	require.Equal(t, 20, settingsBody.Data.TimelinePoints)
}
