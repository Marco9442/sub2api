//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userDisplayRepo struct {
	service.ChannelMonitorRepository
	groups  []service.ChannelMonitorDisplayGroup
	enabled []*service.ChannelMonitor
}

func (r *userDisplayRepo) ListDisplayGroups(context.Context) ([]service.ChannelMonitorDisplayGroup, error) {
	return r.groups, nil
}

func (r *userDisplayRepo) ListEnabled(context.Context) ([]*service.ChannelMonitor, error) {
	return r.enabled, nil
}

func (r *userDisplayRepo) ListLatestForMonitorIDs(context.Context, []int64) (map[int64][]*service.ChannelMonitorLatest, error) {
	return map[int64][]*service.ChannelMonitorLatest{}, nil
}

func (r *userDisplayRepo) ComputeAvailabilityForMonitors(context.Context, []int64, int) (map[int64][]*service.ChannelMonitorAvailability, error) {
	return map[int64][]*service.ChannelMonitorAvailability{}, nil
}

func (r *userDisplayRepo) ListRecentHistoryForMonitors(context.Context, []int64, map[int64]string, int) (map[int64][]*service.ChannelMonitorHistoryEntry, error) {
	return map[int64][]*service.ChannelMonitorHistoryEntry{}, nil
}

type userDisplaySettingRepo struct {
	service.SettingRepository
	values map[string]string
}

func (r userDisplaySettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return r.values, nil
}

func TestUserMonitorListKeepsWindowsAndGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &userDisplayRepo{
		groups: []service.ChannelMonitorDisplayGroup{{ID: 2, Name: "alpha", SortOrder: 0}},
		enabled: []*service.ChannelMonitor{{
			ID:           8,
			Name:         "gpt",
			Provider:     service.MonitorProviderOpenAI,
			GroupName:    "alpha",
			DisplaySort:  3,
			PrimaryModel: "gpt-5.4-mini",
		}},
	}
	handler := NewChannelMonitorUserHandler(service.NewChannelMonitorService(repo, nil), nil)
	router := gin.New()
	router.GET("/channel-monitors", handler.List)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/channel-monitors", nil))

	require.Equal(t, http.StatusOK, response.Code)
	var body struct {
		Data struct {
			Items []struct {
				ID          int64 `json:"id"`
				DisplaySort int   `json:"display_sort"`
			} `json:"items"`
			Groups []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"groups"`
			Windows any `json:"windows"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, int64(8), body.Data.Items[0].ID)
	require.Equal(t, 3, body.Data.Items[0].DisplaySort)
	require.Equal(t, "alpha", body.Data.Groups[0].Name)
	require.Nil(t, body.Data.Windows)
}

func TestUserMonitorListOmitsGroupsWhenProbesAreOff(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := service.NewSettingService(userDisplaySettingRepo{values: map[string]string{
		service.SettingKeyChannelMonitorMode: service.ChannelMonitorModeV2,
	}}, nil)
	handler := NewChannelMonitorUserHandler(service.NewChannelMonitorService(&userDisplayRepo{}, nil), settings)
	router := gin.New()
	router.GET("/channel-monitors", handler.List)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/channel-monitors", nil))

	require.Equal(t, http.StatusOK, response.Code)
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, []any{}, body.Data["items"])
	_, hasGroups := body.Data["groups"]
	require.False(t, hasGroups)
}

func TestUserMonitorListKeepsCustomStatusPercents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := service.NewSettingService(userDisplaySettingRepo{values: map[string]string{
		service.SettingKeyChannelMonitorMode:               service.ChannelMonitorModeCustom,
		service.SettingKeyChannelMonitorCustomStatusYellow: "80",
		service.SettingKeyChannelMonitorCustomStatusRed:    "50",
	}}, nil)
	monitorService := service.NewChannelMonitorService(&userDisplayRepo{}, nil)
	monitorService.SetRuntimeReader(settings)
	handler := NewChannelMonitorUserHandler(monitorService, settings)
	router := gin.New()
	router.GET("/channel-monitors", handler.List)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/channel-monitors", nil))

	require.Equal(t, http.StatusOK, response.Code)
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, float64(80), body.Data["status_yellow_percent"])
	require.Equal(t, float64(50), body.Data["status_red_percent"])
	require.NotNil(t, body.Data["windows"])
	require.NotContains(t, response.Body.String(), "api_key")
	require.NotContains(t, response.Body.String(), "endpoint")
}
