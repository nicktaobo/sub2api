package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"github.com/tidwall/gjson"
)

// 本文件负责从 raw Chat Completions 响应中识别内嵌的生成图片。
//
// 背景：部分 OpenAI 兼容上游（gpt-image-* 经 /v1/chat/completions 直转）把生成
// 结果以 markdown data URI 内嵌在 message.content 里返回：
//
//	![image_1](data:image/png;base64,iVBORw0K...)
//
// 且 usage 以 output_tokens_details.image_tokens 上报图像 token。若不识别，
// ImageCount 恒为 0，计费落 token 分支，绕过分组按张定价（2026-06/07 group 69
// gpt-image-2 少收事故，详见 usage_logs upstream_endpoint=/v1/chat/completions）。
//
// 识别策略（防误判优先——误判会把文本请求按 0.3/张收费，代价高于漏计）：
//  0. 前置门槛：调用方仅对图片模型（isOpenAIImageGenerationModel，gpt-image-*）
//     启用本扫描。普通文本聊天不进入图片计费判定，用户贴图被模型复读也不受影响。
//  1. 喂入的是 gjson 提取后的 content/URL 字符串（已解 JSON 转义，\/ 与 \n
//     不会打断 base64），流式下同一 choice 的 content 分片跨 chunk 续拼。
//  2. `data:image/<subtype>;base64,<payload>`，payload 长度 ≥ chatImageMinBase64Len
//     且满足其一才计为一张图：
//     a) 前缀能解出 PNG/JPEG/GIF 图片头（真实生成图）；
//     b) payload ≥ chatImageHugeBase64Len（stdlib 解不了的 WebP/AVIF 等格式，
//     这个量级的连续 base64 在文本输出里不会自然出现）。
//     文本模型输出里的示例/占位 data URI（favicon、代码示例）过不了这两关。
//  3. 计数封顶 chatImageMaxCount：上游被劫持/异常时不能靠刷 data URI 无限放大
//     客户账单（images 端点有 n≤10 天然约束，这里补等价护栏）。
//  4. 流结束若 payload 仍未闭合，仅在流正常收尾（[DONE]）时计入——中途截断的
//     半张图客户拿到的是废数据，不计费。
//  5. 兜底：未扫到 data URI 但 usage.image_tokens > 0（上游明确上报了图像
//     输出 token，可能以 URL 等其他形式交付）→ 按 1 张计。
//
// 已知接受的局限（对抗审查确认，非缺陷）：
//   - 相邻无分隔的 data URI / 嵌套 marker（`data:image/data:image/...`）等
//     构造性输入可能漏计（状态机不回溯）——真实 markdown 输出不会出现。
//   - 伪造 IHDR 可以谎报尺寸抬价档，但计数封顶后敞口有限，且统一价分组无感。
const (
	chatImageDataURIMarker = "data:image/"
	// 真实生成图的 base64 长度在 10^5 量级；示例/占位 data URI 通常 < 1KB。
	chatImageMinBase64Len = 4096
	// 图片头解码失败时（stdlib 不认的格式），要求的兜底长度。
	chatImageHugeBase64Len = 65536
	// 保留 payload 前缀用于解码尺寸；JPEG 的 SOF 标记一般在头部数 KB 内。
	chatImagePrefixCap = 96 * 1024
	// data URI 头（subtype + ";base64"）的长度上限，超出视为非法放弃匹配。
	chatImageHeaderCap = 64
	// 单次请求可计费的图片数上限（对齐 images 端点 n≤10 的量级，留余量）。
	chatImageMaxCount = 16
)

type chatImageScanState int

const (
	chatImageScanSeeking chatImageScanState = iota // 查找 data:image/ 标记
	chatImageScanHeader                            // 已匹配标记，读取 subtype;base64,
	chatImageScanPayload                           // 逐字符消费 base64 payload
)

// chatImageDataURIScanner 是跨 chunk 的流式 data URI 图片扫描器。
// Feed 按任意切分喂入同一文本单元的连续内容；Break 标记文本单元边界
// （不同字段/不同图片 URL 之间），End 在整个输入结束时收尾。
type chatImageDataURIScanner struct {
	state      chatImageScanState
	carry      string // seeking 态下保留的尾部（防标记跨 chunk 断裂）
	header     string // header 态下已累积的字符
	payloadLen int
	prefix     []byte // payload 前缀（用于解码尺寸），上限 chatImagePrefixCap
	count      int
	sizes      []string
}

func newChatImageDataURIScanner() *chatImageDataURIScanner {
	return &chatImageDataURIScanner{}
}

func isChatImageBase64Char(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '+' || c == '/' || c == '='
}

func isChatImageHeaderChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '.' || c == '+' || c == '-' || c == ';' || c == '='
}

func (s *chatImageDataURIScanner) Feed(chunk string) {
	if s == nil || chunk == "" {
		return
	}
	text := chunk
	if s.carry != "" {
		text = s.carry + chunk
		s.carry = ""
	}
	i := 0
	for i < len(text) {
		switch s.state {
		case chatImageScanSeeking:
			idx := strings.Index(text[i:], chatImageDataURIMarker)
			if idx < 0 {
				// 尾部可能截断了半个标记，保留 len(marker)-1 字符到下次。
				keep := len(chatImageDataURIMarker) - 1
				if remain := len(text) - i; remain < keep {
					keep = remain
				}
				s.carry = text[len(text)-keep:]
				return
			}
			i += idx + len(chatImageDataURIMarker)
			s.state = chatImageScanHeader
			s.header = ""
		case chatImageScanHeader:
			for i < len(text) {
				c := text[i]
				if c == ',' {
					i++
					// RFC 2397 的 "base64" 标记大小写不敏感。
					if len(s.header) >= 7 && strings.EqualFold(s.header[len(s.header)-7:], ";base64") {
						s.state = chatImageScanPayload
						s.payloadLen = 0
						s.prefix = s.prefix[:0]
					} else {
						// 非 base64 data URI（如 svg+xml 明文），不计图。
						s.state = chatImageScanSeeking
					}
					break
				}
				if !isChatImageHeaderChar(c) || len(s.header) >= chatImageHeaderCap {
					s.state = chatImageScanSeeking
					break
				}
				s.header += string(c)
				i++
			}
			// chunk 在 header 中途结束：状态与 header 已保留，下次 Feed 续读。
		case chatImageScanPayload:
			start := i
			for i < len(text) && isChatImageBase64Char(text[i]) {
				i++
			}
			if room := chatImagePrefixCap - len(s.prefix); room > 0 {
				take := i - start
				if take > room {
					take = room
				}
				s.prefix = append(s.prefix, text[start:start+take]...)
			}
			s.payloadLen += i - start
			if i < len(text) {
				// 遇到非 base64 字符，payload 结束。
				s.finalizePayload()
			}
			// chunk 恰在 payload 中途结束：状态保留，下次 Feed 继续消费。
		}
	}
}

// Break 标记一个文本单元的边界（字段之间、URL 之间）：结束当前 payload
// （若已完整满足条件则计入——字段自然结束的裸 data URI 是合法交付），并清空
// 跨界状态，防止不同字段的 base64 被误拼成同一张图。
func (s *chatImageDataURIScanner) Break() {
	if s == nil {
		return
	}
	if s.state == chatImageScanPayload {
		s.finalizePayload()
	}
	s.state = chatImageScanSeeking
	s.carry = ""
	s.header = ""
}

// End 在整个输入流结束后调用。clean=true（正常收尾，如 SSE 收到 [DONE]、
// 非流式完整读完 body）时等价于 Break，会计入停在末尾的裸 data URI；
// clean=false（上游截断）时丢弃未闭合的 payload——半张图不计费。
func (s *chatImageDataURIScanner) End(clean bool) {
	if s == nil {
		return
	}
	if clean {
		s.Break()
		return
	}
	s.payloadLen = 0
	s.prefix = s.prefix[:0]
	s.state = chatImageScanSeeking
	s.carry = ""
	s.header = ""
}

func (s *chatImageDataURIScanner) finalizePayload() {
	if s.payloadLen >= chatImageMinBase64Len && s.count < chatImageMaxCount {
		size := decodeChatImageDims(s.prefix)
		if size != "" || s.payloadLen >= chatImageHugeBase64Len {
			s.count++
			if size != "" {
				s.sizes = append(s.sizes, size)
			}
		}
	}
	s.payloadLen = 0
	s.prefix = s.prefix[:0]
	s.state = chatImageScanSeeking
}

func (s *chatImageDataURIScanner) Count() int {
	if s == nil {
		return 0
	}
	return s.count
}

func (s *chatImageDataURIScanner) Sizes() []string {
	if s == nil || len(s.sizes) == 0 {
		return nil
	}
	return s.sizes
}

// decodeChatImageDims 从 base64 payload 前缀解码图片尺寸，返回 "WxH"；
// 解不出（非 PNG/JPEG/GIF、前缀不足）返回空串，计费侧回落默认档。
func decodeChatImageDims(b64prefix []byte) string {
	n := len(b64prefix) / 4 * 4
	if n == 0 {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(string(b64prefix[:n]))
	if err != nil {
		return ""
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return ""
	}
	return fmt.Sprintf("%dx%d", cfg.Width, cfg.Height)
}

// feedChatImageMessage 提取一个 message 对象里可能承载图片的字段并喂入扫描器：
// content 字符串 / content parts 数组（text、image_url.url）/ images 数组
// （OpenRouter 风格 image_url.url 或 url）。字段之间用 Break 隔断。
func feedChatImageMessage(scanner *chatImageDataURIScanner, msg gjson.Result) {
	if !msg.Exists() {
		return
	}
	if content := msg.Get("content"); content.Type == gjson.String {
		scanner.Feed(content.String())
		scanner.Break()
	} else if content.IsArray() {
		content.ForEach(func(_, part gjson.Result) bool {
			for _, path := range []string{"text", "image_url.url"} {
				if v := part.Get(path); v.Type == gjson.String && v.String() != "" {
					scanner.Feed(v.String())
					scanner.Break()
				}
			}
			return true
		})
	}
	msg.Get("images").ForEach(func(_, img gjson.Result) bool {
		for _, path := range []string{"image_url.url", "url"} {
			if v := img.Get(path); v.Type == gjson.String && v.String() != "" {
				scanner.Feed(v.String())
				scanner.Break()
			}
		}
		return true
	})
}

// detectChatCompletionsImageOutputs 对完整（非流式）响应的 choices 做一次性扫描。
func detectChatCompletionsImageOutputs(choices gjson.Result) (int, []string) {
	if !choices.Exists() {
		return 0, nil
	}
	scanner := newChatImageDataURIScanner()
	choices.ForEach(func(_, choice gjson.Result) bool {
		feedChatImageMessage(scanner, choice.Get("message"))
		return true
	})
	scanner.End(true)
	return scanner.Count(), scanner.Sizes()
}

// feedChatImageStreamChunk 把一个流式 chunk 的 delta 内容喂入按 choice index
// 区分的扫描器集合。同一 choice 的 content 分片跨 chunk 直接续拼（base64 可能
// 被任意切分）,不同 choice 各用独立扫描器,避免交错分片互相污染。
func feedChatImageStreamChunk(scanners map[int64]*chatImageDataURIScanner, payload string) {
	gjson.Get(payload, "choices").ForEach(func(_, choice gjson.Result) bool {
		idx := choice.Get("index").Int()
		scanner := scanners[idx]
		if scanner == nil {
			scanner = newChatImageDataURIScanner()
			scanners[idx] = scanner
		}
		delta := choice.Get("delta")
		if content := delta.Get("content"); content.Type == gjson.String {
			scanner.Feed(content.String()) // 不 Break:分片续拼
		}
		delta.Get("images").ForEach(func(_, img gjson.Result) bool {
			for _, path := range []string{"image_url.url", "url"} {
				if v := img.Get(path); v.Type == gjson.String && v.String() != "" {
					scanner.Feed(v.String())
					scanner.Break()
				}
			}
			return true
		})
		return true
	})
}

// finishChatImageStreamScanners 收尾所有流式扫描器并汇总计数与尺寸。
// 总计数同样封顶 chatImageMaxCount（多 choice 累加不得突破单请求上限）。
func finishChatImageStreamScanners(scanners map[int64]*chatImageDataURIScanner, clean bool) (int, []string) {
	count := 0
	var sizes []string
	for _, scanner := range scanners {
		scanner.End(clean)
		count += scanner.Count()
		sizes = append(sizes, scanner.Sizes()...)
	}
	if count > chatImageMaxCount {
		count = chatImageMaxCount
	}
	return count, sizes
}

// clampCCImageOutputTokens 按 OpenAI 语义（output_tokens_details 是 output_tokens
// 的子集）钳制图像 token。上游伪造超额 image_tokens 会绕过 L3 output cap——
// CapOutputTokens 只压文本部分（reported - imageOutput），图像部分原样放行，
// 不钳制则伪造量直接乘图像 token 价入账（2026-06 gegemini 掺水事件同款手法）。
func clampCCImageOutputTokens(u *OpenAIUsage) {
	if u == nil {
		return
	}
	if u.ImageOutputTokens > u.OutputTokens {
		u.ImageOutputTokens = u.OutputTokens
	}
}

// extractCCSSEFinalUsage 从原始 SSE body 中提取最后一个 usage 对象。
// 供"stream=false 但上游返回 SSE"的聚合路径使用：聚合经 apicompat.ChatUsage
// 往返会丢 output_tokens_details.image_tokens 等细节字段，usage 必须以原始
// SSE 为准，否则 URL 交付的图片丢掉 image_tokens 兜底、退回 token 计费。
func extractCCSSEFinalUsage(body []byte) *OpenAIUsage {
	var out *OpenAIUsage
	forEachOpenAISSEDataPayload(string(body), func(data []byte) {
		if u, ok := openAIUsageFromGJSON(gjson.GetBytes(data, "usage")); ok {
			out = &u
		}
	})
	return out
}
