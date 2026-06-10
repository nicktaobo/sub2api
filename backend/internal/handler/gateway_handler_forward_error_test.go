package handler

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newForwardErrorTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	return c, w
}

// 透传规则命中后 service 层已 c.JSON 写回真实原因，
// handler 不能再补写兜底响应，否则响应体变成 JSON+SSE 帧拼接的非法 JSON，
// 下游网关解析失败后只能展示裸 "bad response status code 400"。
func TestForwardErrorAlreadyCommunicated_PassthroughWritten(t *testing.T) {
	c, w := newForwardErrorTestContext(t)
	writerSizeBeforeForward := c.Writer.Size()

	// 模拟 service 层透传规则命中写回真实原因
	c.JSON(400, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    "invalid_request_error",
			"message": "estimated request context exceeds the selected model's context window",
		},
	})

	err := errors.New("upstream error: 400 (passthrough rule matched) message=estimated request context exceeds the selected model's context window")
	if !forwardErrorAlreadyCommunicated(c, writerSizeBeforeForward, false, err) {
		t.Fatal("expected already-communicated for passthrough-written upstream error")
	}

	body := w.Body.String()
	if !isValidJSONBody(t, body) {
		t.Fatalf("response body should stay valid JSON, got: %q", body)
	}
}

func TestForwardErrorAlreadyCommunicated_ErrorFamilies(t *testing.T) {
	cases := []struct {
		name string
		err  string
		want bool
	}{
		{"claude_passthrough", "upstream error: 400 (passthrough rule matched) message=x", true},
		{"claude_mapped", "upstream error: 429 message=Rate limit", true},
		{"claude_retries_exhausted", "upstream error: 400 (retries exhausted) message=x", true},
		{"gemini_mapped", "gemini upstream error: 400 message=x", true},
		{"antigravity_mapped", "antigravity upstream error: 502", true},
		{"network_error_not_written_by_service", "stream usage incomplete: missing terminal event", false},
		{"plain_request_failed", "upstream request failed: Post \"x\": EOF", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newForwardErrorTestContext(t)
			before := c.Writer.Size()
			// service 层写过响应
			c.JSON(400, gin.H{"type": "error"})
			got := forwardErrorAlreadyCommunicated(c, before, false, errors.New(tc.err))
			if got != tc.want {
				t.Fatalf("err=%q got=%v want=%v", tc.err, got, tc.want)
			}
		})
	}
}

// service 层没写过响应时（writer 无增长），无论错误前缀如何都必须走兜底，
// 否则客户端拿到 silent EOF。
func TestForwardErrorAlreadyCommunicated_NoResponseWritten(t *testing.T) {
	c, _ := newForwardErrorTestContext(t)
	before := c.Writer.Size()
	err := errors.New("upstream error: 400 message=x")
	if forwardErrorAlreadyCommunicated(c, before, false, err) {
		t.Fatal("writer did not grow, fallback response must still be written")
	}
}

// SSE 已开始（ping 已 flush）时 service 写入的 JSON 无法构成合法终止事件，
// 仍需补发 SSE error 帧。
func TestForwardErrorAlreadyCommunicated_StreamStarted(t *testing.T) {
	c, _ := newForwardErrorTestContext(t)
	before := c.Writer.Size()
	c.JSON(400, gin.H{"type": "error"})
	err := errors.New("upstream error: 400 message=x")
	if forwardErrorAlreadyCommunicated(c, before, true, err) {
		t.Fatal("stream already started, SSE error event must still be appended")
	}
}

func isValidJSONBody(t *testing.T, body string) bool {
	t.Helper()
	var v any
	return json.Unmarshal([]byte(body), &v) == nil
}
