package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestUpdateOptionKeepsRootOnlyKeysFromAdmins(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
		role int
		want int
	}{
		{name: "credential", key: "StripeApiSecret", role: common.RoleAdminUser, want: http.StatusForbidden},
		{name: "payment merchant", key: "EpayId", role: common.RoleAdminUser, want: http.StatusForbidden},
		{name: "payment registry setting", key: "payment_setting.amount_options", role: common.RoleAdminUser, want: http.StatusForbidden},
		{name: "sign-in switch", key: "PasswordLoginEnabled", role: common.RoleAdminUser, want: http.StatusForbidden},
		{name: "OAuth provider", key: "oidc.client_id", role: common.RoleAdminUser, want: http.StatusForbidden},
		{name: "outbound mail", key: "SMTPServer", role: common.RoleAdminUser, want: http.StatusForbidden},
		{name: "public address", key: "ServerAddress", role: common.RoleAdminUser, want: http.StatusForbidden},
		// The invalid value is rejected by option validation, which proves the
		// request passed the root-only guard without persisting anything.
		{name: "ordinary setting passes the guard", key: "gemini.safety_settings", role: common.RoleAdminUser, want: http.StatusOK},
		{name: "root is not restricted", key: "gemini.safety_settings", role: common.RoleRootUser, want: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Set("role", tc.role)
			body := fmt.Sprintf(`{"key":%q,"value":"{\"default\":\"BLOCK_SOME\"}"}`, tc.key)
			context.Request = httptest.NewRequest(http.MethodPut, "/api/option/", strings.NewReader(body))

			UpdateOption(context)

			assert.Equal(t, tc.want, response.Code)
		})
	}
}
