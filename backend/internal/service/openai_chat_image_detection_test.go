//go:build unit

package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// testPNGBase64 生成 w×h 的合法 PNG 并补零字节撑过长度阈值后整体 base64。
// 补的是编码前的原始字节,得到的 base64 全程合法、无内部 padding,
// DecodeConfig 只读头部,尺寸解析不受影响。
func testPNGBase64(t *testing.T, w, h, padBytes int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	raw := append(buf.Bytes(), make([]byte, padBytes)...)
	return base64.StdEncoding.EncodeToString(raw)
}

// choicesWithContent 构造非流式 choices JSON(经 json.Marshal,内容会按 JSON
// 规则转义),返回 gjson.Result 供 detectChatCompletionsImageOutputs 使用。
func choicesWithContent(t *testing.T, contents ...string) gjson.Result {
	t.Helper()
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type choice struct {
		Index   int `json:"index"`
		Message msg `json:"message"`
	}
	out := make([]choice, 0, len(contents))
	for i, content := range contents {
		out = append(out, choice{Index: i, Message: msg{Role: "assistant", Content: content}})
	}
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	return gjson.ParseBytes(raw)
}

func TestChatImageDetect_CountsMarkdownDataURIImage(t *testing.T) {
	t.Parallel()
	payload := testPNGBase64(t, 4, 7, 8192)
	choices := choicesWithContent(t, fmt.Sprintf("![image_1](data:image/png;base64,%s)", payload))

	count, sizes := detectChatCompletionsImageOutputs(choices)
	require.Equal(t, 1, count)
	require.Equal(t, []string{"4x7"}, sizes)
}

func TestChatImageDetect_CountsMultipleImagesAcrossChoices(t *testing.T) {
	t.Parallel()
	p1 := testPNGBase64(t, 2, 3, 8192)
	p2 := testPNGBase64(t, 5, 6, 8192)
	choices := choicesWithContent(t,
		fmt.Sprintf("![a](data:image/png;base64,%s) and ![b](data:image/png;base64,%s)", p1, p1),
		fmt.Sprintf("second choice ![c](data:image/png;base64,%s)", p2),
	)

	count, sizes := detectChatCompletionsImageOutputs(choices)
	require.Equal(t, 3, count)
	require.Equal(t, []string{"2x3", "2x3", "5x6"}, sizes)
}

func TestChatImageDetect_JSONEscapedSolidusStillDetected(t *testing.T) {
	t.Parallel()
	// PHP json_encode 等默认把 / 转义成 \/ ——marker 和 base64 里的 / 全部
	// 变形。喂入 gjson 提取后的字符串(已解转义)必须不受影响。
	payload := testPNGBase64(t, 3, 5, 8192)
	content := "![img](data:image/png;base64," + payload + ")"
	escaped := strings.ReplaceAll(content, "/", `\/`)
	raw := `[{"index":0,"message":{"role":"assistant","content":"` + escaped + `"}}]`
	require.True(t, gjson.Valid(raw))

	count, sizes := detectChatCompletionsImageOutputs(gjson.Parse(raw))
	require.Equal(t, 1, count)
	require.Equal(t, []string{"3x5"}, sizes)
}

func TestChatImageDetect_ContentPartsAndImagesArray(t *testing.T) {
	t.Parallel()
	payload := testPNGBase64(t, 7, 7, 8192)
	// OpenRouter 风格:message.images[].image_url.url;content 为 parts 数组。
	raw := `[{"index":0,"message":{"role":"assistant","content":[{"type":"text","text":"here"}],"images":[{"type":"image_url","image_url":{"url":"data:image/png;base64,` + payload + `"}}]}}]`
	count, sizes := detectChatCompletionsImageOutputs(gjson.Parse(raw))
	require.Equal(t, 1, count)
	require.Equal(t, []string{"7x7"}, sizes)
}

func TestChatImageDetect_CountCappedAtMax(t *testing.T) {
	t.Parallel()
	payload := testPNGBase64(t, 2, 2, 8192)
	one := "![x](data:image/png;base64," + payload + ")"
	content := strings.Repeat(one, chatImageMaxCount+8)
	count, sizes := detectChatCompletionsImageOutputs(choicesWithContent(t, content))
	require.Equal(t, chatImageMaxCount, count)
	require.Len(t, sizes, chatImageMaxCount)
}

func TestChatImageScanner_IgnoresTinyDataURI(t *testing.T) {
	t.Parallel()
	// 典型的代码示例/favicon 占位:远小于阈值,不计。
	scanner := newChatImageDataURIScanner()
	scanner.Feed(`<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==">`)
	scanner.End(true)
	require.Zero(t, scanner.Count())
	require.Nil(t, scanner.Sizes())
}

func TestChatImageScanner_IgnoresLargeUndecodableUnderHugeThreshold(t *testing.T) {
	t.Parallel()
	// ≥4KB 但解不出图片头且不到 64KB:视为文本里的 base64 块,不计。
	scanner := newChatImageDataURIScanner()
	scanner.Feed("data:image/png;base64," + strings.Repeat("QUJD", 2048)) // 8192 字符
	scanner.End(true)
	require.Zero(t, scanner.Count())
}

func TestChatImageScanner_CountsHugeUndecodablePayload(t *testing.T) {
	t.Parallel()
	// stdlib 解不了的格式(如 WebP):≥64KB 兜底计入,无尺寸。
	scanner := newChatImageDataURIScanner()
	scanner.Feed("data:image/webp;base64," + strings.Repeat("QUJD", 20000)) // 80000 字符
	scanner.End(true)
	require.Equal(t, 1, scanner.Count())
	require.Nil(t, scanner.Sizes())
}

func TestChatImageScanner_IgnoresNonBase64DataURI(t *testing.T) {
	t.Parallel()
	scanner := newChatImageDataURIScanner()
	scanner.Feed(`data:image/svg+xml,%3Csvg%20xmlns%3D` + strings.Repeat("A", 8192))
	scanner.End(true)
	require.Zero(t, scanner.Count())
}

func TestChatImageScanner_Base64MarkerCaseInsensitive(t *testing.T) {
	t.Parallel()
	payload := testPNGBase64(t, 4, 4, 8192)
	scanner := newChatImageDataURIScanner()
	scanner.Feed("data:image/png;BASE64," + payload)
	scanner.End(true)
	require.Equal(t, 1, scanner.Count())
}

func TestChatImageScanner_HandlesChunkSplits(t *testing.T) {
	t.Parallel()
	payload := testPNGBase64(t, 3, 9, 8192)
	full := fmt.Sprintf("![image_1](data:image/png;base64,%s)", payload)

	// 在各种恶劣位置切分:标记中间、header 中间、payload 中间。
	for _, splits := range [][]int{
		{15},                  // "data:ima|ge/" 附近
		{25, 26},              // ";base64," 附近逐字符
		{100, 5000, 5001},     // payload 内多次
		{1, 2, 3, 4, 5, 6, 7}, // 开头逐字符
		{len(full) - 3},       // 收尾前
	} {
		scanner := newChatImageDataURIScanner()
		prev := 0
		for _, at := range splits {
			if at > len(full) {
				at = len(full)
			}
			scanner.Feed(full[prev:at])
			prev = at
		}
		scanner.Feed(full[prev:])
		scanner.End(true)
		require.Equal(t, 1, scanner.Count(), "splits=%v", splits)
		require.Equal(t, []string{"3x9"}, scanner.Sizes(), "splits=%v", splits)
	}
}

func TestChatImageScanner_CleanEndCountsTrailingPayload(t *testing.T) {
	t.Parallel()
	payload := testPNGBase64(t, 8, 8, 8192)
	scanner := newChatImageDataURIScanner()
	scanner.Feed("data:image/png;base64," + payload) // 裸 data URI 收尾,无分隔符
	scanner.End(true)
	require.Equal(t, 1, scanner.Count())
	require.Equal(t, []string{"8x8"}, scanner.Sizes())
}

func TestChatImageScanner_TruncatedEndDiscardsPartialPayload(t *testing.T) {
	t.Parallel()
	// 上游截断:payload 前缀已够长、图片头也能解出,但流没有正常收尾——
	// 客户拿到的是半张废图,不得计费。
	payload := testPNGBase64(t, 1024, 1024, 200000)
	scanner := newChatImageDataURIScanner()
	scanner.Feed("![img](data:image/png;base64," + payload[:6000])
	scanner.End(false)
	require.Zero(t, scanner.Count())
	require.Nil(t, scanner.Sizes())
}

func TestChatImageScanner_BreakSeparatesFields(t *testing.T) {
	t.Parallel()
	// 两个字段各含半截 base64(都不足阈值):Break 隔断后不得拼成一张图。
	half := strings.Repeat("QUJE", 600) // 2400 字符 < 4096
	scanner := newChatImageDataURIScanner()
	scanner.Feed("data:image/png;base64," + half)
	scanner.Break()
	scanner.Feed(half + half) // 无 marker 的裸 base64,不应续上前一字段
	scanner.End(true)
	require.Zero(t, scanner.Count())
}

func TestChatImageStreamChunks_FragmentedAcrossDeltas(t *testing.T) {
	t.Parallel()
	// 真实流式形态:一张图的 markdown data URI 被切成许多小段 delta.content,
	// 每段是独立的 chunk JSON。检测必须跨 chunk 续拼出完整 base64。
	payload := testPNGBase64(t, 6, 3, 8192)
	content := "![image_1](data:image/png;base64," + payload + ")"
	scanners := map[int64]*chatImageDataURIScanner{}
	for start := 0; start < len(content); start += 512 {
		end := start + 512
		if end > len(content) {
			end = len(content)
		}
		chunk, err := json.Marshal(map[string]any{
			"choices": []map[string]any{
				{"index": 0, "delta": map[string]any{"content": content[start:end]}},
			},
		})
		require.NoError(t, err)
		feedChatImageStreamChunk(scanners, string(chunk))
	}
	count, sizes := finishChatImageStreamScanners(scanners, true)
	require.Equal(t, 1, count)
	require.Equal(t, []string{"6x3"}, sizes)
}

func TestChatImageStreamChunks_MultiImageFragmented(t *testing.T) {
	t.Parallel()
	p1 := testPNGBase64(t, 2, 9, 8192)
	p2 := testPNGBase64(t, 9, 2, 8192)
	content := "![a](data:image/png;base64," + p1 + ") ![b](data:image/png;base64," + p2 + ")"
	scanners := map[int64]*chatImageDataURIScanner{}
	for start := 0; start < len(content); start += 1024 {
		end := start + 1024
		if end > len(content) {
			end = len(content)
		}
		chunk, err := json.Marshal(map[string]any{
			"choices": []map[string]any{
				{"index": 0, "delta": map[string]any{"content": content[start:end]}},
			},
		})
		require.NoError(t, err)
		feedChatImageStreamChunk(scanners, string(chunk))
	}
	count, sizes := finishChatImageStreamScanners(scanners, true)
	require.Equal(t, 2, count)
	require.Equal(t, []string{"2x9", "9x2"}, sizes)
}

func TestClampCCImageOutputTokens(t *testing.T) {
	t.Parallel()
	u := OpenAIUsage{OutputTokens: 100, ImageOutputTokens: 2_000_000}
	clampCCImageOutputTokens(&u)
	require.Equal(t, 100, u.ImageOutputTokens)

	normal := OpenAIUsage{OutputTokens: 1584, ImageOutputTokens: 1584}
	clampCCImageOutputTokens(&normal)
	require.Equal(t, 1584, normal.ImageOutputTokens)
}

func TestExtractCCSSEFinalUsage_KeepsImageTokens(t *testing.T) {
	t.Parallel()
	sse := strings.Join([]string{
		`data: {"choices":[{"index":0,"delta":{"content":"x"}}]}`,
		"",
		`data: {"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":1584,"output_tokens_details":{"image_tokens":1584}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	u := extractCCSSEFinalUsage([]byte(sse))
	require.NotNil(t, u)
	require.Equal(t, 5, u.InputTokens)
	require.Equal(t, 1584, u.OutputTokens)
	require.Equal(t, 1584, u.ImageOutputTokens)
}
