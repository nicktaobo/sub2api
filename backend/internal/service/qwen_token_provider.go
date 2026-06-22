package service

import (
	"context"
	"errors"
	"strings"
)

// QwenTokenProvider manages Qwen (通义千问 / Alibaba DashScope) account credentials.
// Currently only API Key mode is supported.
type QwenTokenProvider struct{}

func NewQwenTokenProvider() *QwenTokenProvider {
	return &QwenTokenProvider{}
}

func (p *QwenTokenProvider) GetAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is nil")
	}
	if account.Platform != PlatformQwen {
		return "", errors.New("not a qwen account")
	}

	if account.Type == AccountTypeAPIKey {
		token := account.GetCredential("api_key")
		if strings.TrimSpace(token) == "" {
			return "", errors.New("api_key not found in credentials")
		}
		return token, nil
	}

	return "", errors.New("qwen: unsupported account type")
}
