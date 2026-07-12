package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// staleWriterWrapper 模拟下游 handler 把 c.Writer 换成新的包装器且未恢复的
// 场景（如 compact SSE keepalive writer 残留），包装器继续引用内层的池化
// opsCaptureWriter。
type staleWriterWrapper struct {
	gin.ResponseWriter
}

// 回归：下游残留 wrapper 替换 c.Writer 后中间件退出时，必须无条件恢复
// originalWriter，且被残留 wrapper 引用的池化 writer 不得归还 sync.Pool
// （否则池复用后是跨请求 use-after-release 数据竞争）。
func TestOpsErrorLoggerMiddleware_ResidualWrapperRestoresWriterAndSkipsPool(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var (
		originalWriter gin.ResponseWriter
		writerAfterOps gin.ResponseWriter
		pooled         *opsCaptureWriter
	)

	r := gin.New()
	// 外层中间件模拟全局 Logger/Recovery：post-Next 仍会读 c.Writer。
	r.Use(func(c *gin.Context) {
		originalWriter = c.Writer
		c.Next()
		writerAfterOps = c.Writer
	})
	r.Use(OpsErrorLoggerMiddleware(nil))
	r.GET("/x", func(c *gin.Context) {
		var ok bool
		pooled, ok = c.Writer.(*opsCaptureWriter)
		require.True(t, ok, "ops 中间件应已安装池化捕获 writer")
		// 模拟 keepalive wrapper 替换 c.Writer 且全程未恢复。
		c.Writer = &staleWriterWrapper{ResponseWriter: c.Writer}
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	require.NotNil(t, pooled)
	// (1) 无条件恢复：外层中间件 post-Next 看到的必须是原始 writer。
	require.Same(t, originalWriter, writerAfterOps, "存在残留 wrapper 时也必须恢复 originalWriter")
	// (2) 未入池：releaseOpsCaptureWriter 是唯一入池路径且会把 ResponseWriter
	// 置 nil；ResponseWriter 仍非 nil 即证明 w 被丢给 GC 而非归还池。
	require.NotNil(t, pooled.ResponseWriter, "被残留 wrapper 引用的 writer 不得归还 sync.Pool")
}

// 常规路径（无残留 wrapper）不回归：中间件退出恢复原 writer 并正常归还池。
func TestOpsErrorLoggerMiddleware_NormalPathRestoresWriterAndReleasesToPool(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var (
		originalWriter gin.ResponseWriter
		writerAfterOps gin.ResponseWriter
		pooled         *opsCaptureWriter
	)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		originalWriter = c.Writer
		c.Next()
		writerAfterOps = c.Writer
	})
	r.Use(OpsErrorLoggerMiddleware(nil))
	r.GET("/x", func(c *gin.Context) {
		var ok bool
		pooled, ok = c.Writer.(*opsCaptureWriter)
		require.True(t, ok)
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	require.NotNil(t, pooled)
	require.Same(t, originalWriter, writerAfterOps)
	require.Nil(t, pooled.ResponseWriter, "常规路径应归还池（release 会清空内层引用）")
}
