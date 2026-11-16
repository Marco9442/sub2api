package accountsim

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// Handler 提供账号管理 #2 的全部接口，挂在 /admin/account-sim/*path 下。
// 路径与真实管理接口一一对应（去掉 /account-sim 前缀即为真实路径），响应结构复用真实 DTO。
// 只注册一个通配路由，由这里分发，未覆盖的接口统一返回安全默认值。
type Handler struct {
	svc    *Service
	routes []route
}

type route struct {
	method string
	parts  []string
	fn     func(c *gin.Context, p params)
}

// params 是路径里 :name 段的取值。
type params map[string]string

// id 解析 :id 段，失败时已写入 400 响应。
func (p params) id(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(p["id"], 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return 0, false
	}
	return id, true
}

// NewHandler 创建接口层。
func NewHandler(svc *Service) *Handler {
	h := &Handler{svc: svc}
	h.registerRoutes()
	return h
}

func (h *Handler) handle(method, pattern string, fn func(c *gin.Context, p params)) {
	h.routes = append(h.routes, route{method: method, parts: splitPath(pattern), fn: fn})
}

// Serve 按方法和路径分发请求。具体路径优先于带参数的路径，与 gin 的匹配顺序一致。
func (h *Handler) Serve(c *gin.Context) {
	parts := splitPath(c.Param("path"))
	var best *route
	var bestParams params
	bestScore := -1
	for i := range h.routes {
		r := &h.routes[i]
		if r.method != c.Request.Method {
			continue
		}
		if p, score, ok := matchRoute(r.parts, parts); ok && score > bestScore {
			best, bestParams, bestScore = r, p, score
		}
	}
	if best == nil {
		fallback(c)
		return
	}
	best.fn(c, bestParams)
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

// matchRoute 返回参数与匹配得分，字面段越多得分越高。
func matchRoute(pattern, parts []string) (params, int, bool) {
	if len(pattern) != len(parts) {
		return nil, 0, false
	}
	p := params{}
	score := 0
	for i, seg := range pattern {
		if strings.HasPrefix(seg, ":") {
			p[seg[1:]] = parts[i]
			continue
		}
		if seg != parts[i] {
			return nil, 0, false
		}
		score++
	}
	return p, score, true
}

// fallback 是上游新增而这里尚未覆盖的接口的安全默认值：成功、空数据、不写状态。
func fallback(c *gin.Context) {
	response.Success(c, gin.H{})
}

func notFound(c *gin.Context) {
	response.NotFound(c, "Account not found")
}

// bind 解析 JSON 请求体，失败时已写入 400 响应。
func bind(c *gin.Context, out any) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return false
	}
	return true
}

func internalError(c *gin.Context, err error) {
	response.Error(c, http.StatusInternalServerError, err.Error())
}
