package service

import (
	"context"
	"errors"
	"strings"
)

var ErrDeepSeekOAuthNotImplemented = errors.New("deepseek OAuth not implemented")

// DeepSeekTokenProvider manages DeepSeek account credentials.
// Currently only API Key mode is supported; OAuth is reserved.
type DeepSeekTokenProvider struct{}

func NewDeepSeekTokenProvider() *DeepSeekTokenProvider {
	return &DeepSeekTokenProvider{}
}

func (p *DeepSeekTokenProvider) GetAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is nil")
	}
	if account.Platform != PlatformDeepSeek {
		return "", errors.New("not a deepseek account")
	}

	if account.Type == AccountTypeAPIKey {
		token := account.GetCredential("api_key")
		if strings.TrimSpace(token) == "" {
			return "", errors.New("api_key not found in credentials")
		}
		return token, nil
	}

	return "", ErrDeepSeekOAuthNotImplemented
}
