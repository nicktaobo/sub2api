package service

import (
	"context"
	"errors"
	"strings"
)

// GLMTokenProvider manages GLM (Zhipu AI) account credentials.
// Currently only API Key mode is supported.
type GLMTokenProvider struct{}

func NewGLMTokenProvider() *GLMTokenProvider {
	return &GLMTokenProvider{}
}

func (p *GLMTokenProvider) GetAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is nil")
	}
	if account.Platform != PlatformGLM {
		return "", errors.New("not a glm account")
	}

	if account.Type == AccountTypeAPIKey {
		token := account.GetCredential("api_key")
		if strings.TrimSpace(token) == "" {
			return "", errors.New("api_key not found in credentials")
		}
		return token, nil
	}

	return "", errors.New("glm: unsupported account type")
}
