//go:build unit

package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestAccountTestService_GrokProbesXAINotAnthropic locks in that a Grok account's
// connection test probes the xAI /responses endpoint with a Bearer access token,
// instead of falling through to the Anthropic Messages probe (api.anthropic.com).
func TestAccountTestService_GrokProbesXAINotAnthropic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	account := &Account{
		ID:          77,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "grok-test-token",
			"base_url":     "https://api.x.ai/v1",
		},
	}

	repo := &mockAccountRepoForGemini{
		accountsByID: map[int64]*Account{account.ID: account},
	}
	upstream := &queuedHTTPUpstream{
		responses: []*http.Response{newJSONResponse(http.StatusOK, `{"id":"resp_1","status":"completed"}`)},
	}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}

	err := svc.TestAccountConnection(ctx, account.ID, "", "", "")
	require.NoError(t, err)

	require.Len(t, upstream.requests, 1)
	captured := upstream.requests[0]

	// Must hit xAI, never Anthropic.
	require.Equal(t, "api.x.ai", captured.URL.Host)
	require.NotContains(t, captured.URL.Host, "anthropic.com")
	require.True(t, strings.HasSuffix(captured.URL.Path, "/responses"),
		"expected xAI responses path, got %q", captured.URL.Path)

	// Authorization header must carry the OAuth access token as a Bearer token.
	require.Equal(t, "Bearer grok-test-token", captured.Header.Get("Authorization"))

	require.Contains(t, recorder.Body.String(), "test_complete")
}
