//go:build unit

package service

import (
	"reflect"
	"testing"
)

func TestClaudeUsageSanitizeNegatives(t *testing.T) {
	u := ClaudeUsage{
		InputTokens:              100,
		OutputTokens:             -50,
		CacheCreationInputTokens: -1,
		CacheReadInputTokens:     200,
		CacheCreation5mTokens:    0,
		CacheCreation1hTokens:    -999999,
		ImageOutputTokens:        7,
	}
	clamped := u.SanitizeNegatives()

	want := []string{"output_tokens", "cache_creation_input_tokens", "cache_creation_1h_tokens"}
	if !reflect.DeepEqual(clamped, want) {
		t.Fatalf("clamped fields = %v, want %v", clamped, want)
	}
	if u.OutputTokens != 0 || u.CacheCreationInputTokens != 0 || u.CacheCreation1hTokens != 0 {
		t.Fatalf("negative fields not zeroed: %+v", u)
	}
	// 非负字段不受影响
	if u.InputTokens != 100 || u.CacheReadInputTokens != 200 || u.ImageOutputTokens != 7 {
		t.Fatalf("non-negative fields changed: %+v", u)
	}
}

func TestClaudeUsageSanitizeNegativesNoop(t *testing.T) {
	u := ClaudeUsage{InputTokens: 1, OutputTokens: 2}
	if clamped := u.SanitizeNegatives(); clamped != nil {
		t.Fatalf("expected nil for all non-negative usage, got %v", clamped)
	}
	if u.InputTokens != 1 || u.OutputTokens != 2 {
		t.Fatalf("usage mutated on noop: %+v", u)
	}
}

func TestOpenAIUsageSanitizeNegatives(t *testing.T) {
	u := OpenAIUsage{
		InputTokens:          -3,
		ImageInputTokens:     5,
		OutputTokens:         10,
		CacheReadInputTokens: -8,
	}
	clamped := u.SanitizeNegatives()

	want := []string{"input_tokens", "cache_read_input_tokens"}
	if !reflect.DeepEqual(clamped, want) {
		t.Fatalf("clamped fields = %v, want %v", clamped, want)
	}
	if u.InputTokens != 0 || u.CacheReadInputTokens != 0 {
		t.Fatalf("negative fields not zeroed: %+v", u)
	}
	if u.ImageInputTokens != 5 || u.OutputTokens != 10 {
		t.Fatalf("non-negative fields changed: %+v", u)
	}
}
