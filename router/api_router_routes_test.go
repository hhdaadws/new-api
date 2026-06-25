package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApiRouterIncludesMergedFeatureRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	registered := make(map[string]bool)
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	expectedRoutes := []string{
		"GET /api/commission/",
		"POST /api/commission/approve",
		"POST /api/commission/reject",
		"POST /api/commission/batch/approve",
		"POST /api/commission/batch/reject",

		"GET /api/group_shard/",
		"POST /api/group_shard/",
		"PUT /api/group_shard/",
		"DELETE /api/group_shard/:id",
		"POST /api/group_shard/recount",
		"POST /api/group_shard/assign_user",

		"POST /api/ticket/",
		"GET /api/ticket/self",
		"GET /api/ticket/self/search",
		"GET /api/ticket/self/:id",
		"POST /api/ticket/self/:id/message",
		"POST /api/ticket/self/:id/close",
		"POST /api/ticket/self/:id/rate",
		"GET /api/ticket/",
		"GET /api/ticket/search",
		"GET /api/ticket/stats",
		"GET /api/ticket/admins",
		"GET /api/ticket/:id",
		"PUT /api/ticket/:id/status",
		"PUT /api/ticket/:id/assign",
		"POST /api/ticket/:id/message",

		"GET /api/channel/:id/user_bindings",
		"DELETE /api/channel/:id/user_bindings/:user_id",
		"DELETE /api/channel/:id/user_bindings",
		"GET /api/channel/:id/session_bindings",
		"DELETE /api/channel/:id/session_bindings/:session_id",
		"DELETE /api/channel/:id/session_bindings",
		"GET /api/channel/:id/session_spoof",
		"POST /api/channel/analyze_users",
		"POST /api/channel/:id/user_bindings/batch",

		"POST /api/user/waffo-pancake/amount",
		"POST /api/user/waffo-pancake/pay",
		"POST /api/waffo-pancake/webhook",
	}

	for _, route := range expectedRoutes {
		require.Truef(t, registered[route], "expected route %s to be registered", route)
	}
}
