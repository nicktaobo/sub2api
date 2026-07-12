//go:build unit

package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// batchImageOwnerFromContext 必须把 apiKey.User.ParentMerchantID 透传给 owner，
// Submit 侧依赖该字段拒绝 merchant 子用户（batch_image 结算不走 merchant
// 加价/返佣/ledger，见 service.ErrBatchImageMerchantSubUserForbidden）。
func TestBatchImageOwnerFromContextCarriesParentMerchantID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newCtxWithAPIKey := func(apiKey *service.APIKey) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		return c
	}

	t.Run("merchant sub user carries parent merchant id", func(t *testing.T) {
		merchantID := int64(9)
		groupID := int64(7)
		c := newCtxWithAPIKey(&service.APIKey{
			ID:      22,
			UserID:  11,
			GroupID: &groupID,
			User:    &service.User{ID: 11, ParentMerchantID: &merchantID},
		})

		owner, ok := batchImageOwnerFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(11), owner.UserID)
		require.Equal(t, int64(22), owner.APIKeyID)
		require.NotNil(t, owner.ParentMerchantID)
		require.Equal(t, merchantID, *owner.ParentMerchantID)
	})

	t.Run("normal user has nil parent merchant id", func(t *testing.T) {
		c := newCtxWithAPIKey(&service.APIKey{
			ID:     22,
			UserID: 11,
			User:   &service.User{ID: 11},
		})

		owner, ok := batchImageOwnerFromContext(c)
		require.True(t, ok)
		require.Nil(t, owner.ParentMerchantID)
	})

	t.Run("missing api key still rejected", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		_, ok := batchImageOwnerFromContext(c)
		require.False(t, ok)
	})
}
