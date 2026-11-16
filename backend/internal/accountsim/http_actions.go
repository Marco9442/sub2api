package accountsim

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) testConnection(c *gin.Context, p params) {
	id, ok := p.id(c)
	if !ok {
		return
	}
	var model string
	found := false
	err := h.svc.View(c.Request.Context(), func(state *State, _ Config, _ time.Time) {
		account := state.Accounts[id]
		if account == nil {
			return
		}
		found = true
		model = platformTestModel(account.Platform)
	})
	if err != nil {
		internalError(c, err)
		return
	}
	if !found {
		notFound(c)
		return
	}
	writeTestStream(c, model)
}

func platformTestModel(platform string) string {
	if models := platformModels[platform]; len(models) > 0 {
		return models[0].Name
	}
	return "gpt-5.5"
}

func writeTestStream(c *gin.Context, model string) {
	header := c.Writer.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	send := func(event map[string]any) {
		raw, _ := json.Marshal(event)
		fmt.Fprintf(c.Writer, "data: %s\n\n", raw)
	}
	send(map[string]any{"type": "test_start", "model": model})
	send(map[string]any{"type": "content", "text": "Hello."})
	send(map[string]any{"type": "test_complete", "success": true})
	c.Writer.Flush()
}

type claudeModelDTO struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
}

func (h *Handler) models(c *gin.Context, p params) {
	id, ok := p.id(c)
	if !ok {
		return
	}
	h.read(c, func(v view) {
		account := v.state.Accounts[id]
		if account == nil {
			notFound(c)
			return
		}
		response.Success(c, platformClaudeModels(account.Platform))
	})
}

func (h *Handler) syncModels(c *gin.Context, p params) {
	id, ok := p.id(c)
	if !ok {
		return
	}
	h.read(c, func(v view) {
		account := v.state.Accounts[id]
		if account == nil {
			notFound(c)
			return
		}
		models := platformClaudeModels(account.Platform)
		names := make([]string, len(models))
		for i := range models {
			names[i] = models[i].ID
		}
		response.Success(c, gin.H{"models": names, "discovery_source": "configured_fallback"})
	})
}

func platformClaudeModels(platform string) []claudeModelDTO {
	src := platformModels[platform]
	out := make([]claudeModelDTO, 0, len(src))
	for _, model := range src {
		out = append(out, claudeModelDTO{
			ID: model.Name, Type: "model", DisplayName: model.Name, CreatedAt: "2026-01-01T00:00:00Z",
		})
	}
	return out
}
