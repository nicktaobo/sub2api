package service

import (
	"context"
	"errors"
	"strings"
)

var ErrMoonshotOAuthNotImplemented = errors.New("moonshot OAuth not implemented")

// MoonshotTokenProvider manages Moonshot (Kimi) account credentials.
// Currently only API Key mode is supported; OAuth is reserved.
type MoonshotTokenProvider struct{}

func NewMoonshotTokenProvider() *MoonshotTokenProvider {
	return &MoonshotTokenProvider{}
}

func (p *MoonshotTokenProvider) GetAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is nil")
	}
	if account.Platform != PlatformMoonshot {
		return "", errors.New("not a moonshot account")
	}

	if account.Type == AccountTypeAPIKey {
		token := account.GetCredential("api_key")
		if strings.TrimSpace(token) == "" {
			return "", errors.New("api_key not found in credentials")
		}
		return token, nil
	}

	return "", ErrMoonshotOAuthNotImplemented
}
