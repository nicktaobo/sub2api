package service

import (
	"context"
	"errors"
	"strings"
)

// SeedanceTokenProvider manages Seedance (ByteDance/Volcano Ark) account credentials.
// Currently only API Key mode is supported.
type SeedanceTokenProvider struct{}

func NewSeedanceTokenProvider() *SeedanceTokenProvider {
	return &SeedanceTokenProvider{}
}

func (p *SeedanceTokenProvider) GetAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is nil")
	}
	if account.Platform != PlatformSeedance {
		return "", errors.New("not a seedance account")
	}

	if account.Type == AccountTypeAPIKey {
		token := account.GetCredential("api_key")
		if strings.TrimSpace(token) == "" {
			return "", errors.New("api_key not found in credentials")
		}
		return token, nil
	}

	return "", errors.New("seedance: unsupported account type")
}
