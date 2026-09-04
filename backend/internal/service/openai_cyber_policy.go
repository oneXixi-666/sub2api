package service

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// opsCyberPolicyKey 在 gin context 中携带 cyber_policy 命中标记。
// 由 gateway 服务层在检测到上游 error.code=="cyber_policy" 时设置，
// handler 在 Forward 返回后读取以触发风控记录、邮件与 tokens=0 用量行。
const opsCyberPolicyKey = "ops_cyber_policy"

// errOpenAICyberPolicyForwarded 表示 cyber_policy 已按当前端点格式透传给客户端
// （error 已写出/下发）。compat 路径 ForwardAsChatCompletions / ForwardAsAnthropic 出口
// 据此丢弃 result 并返回该哨兵，使 handler 落入 tokens=0 免费用量行（对齐 /v1/responses），
// 既不计费、也不 failover、不重复写响应。
var errOpenAICyberPolicyForwarded = errors.New("openai cyber_policy forwarded to client")

// CyberPolicyMark 记录一次 cyber_policy 硬阻断的上游证据。
type CyberPolicyMark struct {
	Code           string                  // 固定 "cyber_policy"
	Message        string                  // 上游 error.message
	Body           string                  // 上游 response.failed / 400 原始 body（已截断；未脱敏，ops_error 落库由 sanitizeErrorBodyForStorage、风控日志由 redactContentModerationSecrets 统一脱敏）
	UpstreamStatus int                     // 上游 HTTP 状态（流式=200，非流式=400）
	UpstreamInTok  int                     // 上游已报 input tokens（如有）
	UpstreamOutTok int                     // 上游已报 output tokens（如有）
	InputOffset    *CyberPolicyInputOffset // 上游提供的归一化输入命中范围（可选）
}

// CyberPolicyInputOffset identifies the upstream-reported match range in the
// normalized conversation text (rune offsets, end exclusive). It is optional:
// older upstream responses expose only the cyber_policy code/message.
type CyberPolicyInputOffset struct {
	Start int
	End   int
}

// MarkOpsCyberPolicy 记录 cyber 标记；首个写入生效，后续忽略（同一 turn 只记一次）。
// WS 多轮场景由 handler 在每个 turn 结束后调用 ClearOpsCyberPolicy 重置。
func MarkOpsCyberPolicy(c *gin.Context, mark CyberPolicyMark) {
	if c == nil {
		return
	}
	if GetOpsCyberPolicy(c) != nil {
		return
	}
	mark.Code = "cyber_policy"
	mark.Message = strings.TrimSpace(mark.Message)
	mark.Body = strings.TrimSpace(mark.Body)
	if mark.InputOffset == nil {
		if offset, ok := extractOpenAICyberPolicyInputOffset([]byte(mark.Body)); ok {
			mark.InputOffset = &offset
		}
	} else {
		offset := *mark.InputOffset
		if normalized, ok := normalizeCyberPolicyInputOffset(offset.Start, offset.End); ok {
			mark.InputOffset = &normalized
		} else {
			mark.InputOffset = nil
		}
	}
	c.Set(opsCyberPolicyKey, &mark)
}

// GetOpsCyberPolicy 返回 cyber 标记，未命中（或已被 Clear）返回 nil。
func GetOpsCyberPolicy(c *gin.Context) *CyberPolicyMark {
	if c == nil {
		return nil
	}
	if v, ok := c.Get(opsCyberPolicyKey); ok {
		if m, ok := v.(*CyberPolicyMark); ok && m != nil {
			return m
		}
	}
	return nil
}

// ClearOpsCyberPolicy 清除 cyber 标记（typed-nil 覆盖；gin context 无并发安全的
// 删除原语，Set 走内部锁，与异步 GetOpsCyberPolicy 不构成 data race）。
// 仅 WS 多轮路径在 turn 收尾调用；HTTP 单请求路径不调用（context 随请求销毁，
// 且中间件 shouldSkipOpsErrorLogForCyber 依赖标记防双写）。
// WS 路径 clear 发生在中间件收尾之前，连接响应状态为 101，不触发中间件 status>=400
// 落库分支，故无双写/漏写。
func ClearOpsCyberPolicy(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(opsCyberPolicyKey, (*CyberPolicyMark)(nil))
}

// detectOpenAICyberPolicy 精确识别 cyber_policy（对齐 codex api_bridge.rs:145 /
// sse/responses.rs:529）。命中返回 (true, "cyber_policy", message)。
func detectOpenAICyberPolicy(payload []byte) (bool, string, string) {
	code := gjson.GetBytes(payload, "error.code").String()
	if code == "" {
		code = gjson.GetBytes(payload, "response.error.code").String()
	}
	if !strings.EqualFold(strings.TrimSpace(code), "cyber_policy") {
		return false, "", ""
	}
	msg := gjson.GetBytes(payload, "error.message").String()
	if msg == "" {
		msg = gjson.GetBytes(payload, "response.error.message").String()
	}
	return true, "cyber_policy", strings.TrimSpace(msg)
}

func extractOpenAICyberPolicyInputOffset(payload []byte) (CyberPolicyInputOffset, bool) {
	if len(payload) == 0 {
		return CyberPolicyInputOffset{}, false
	}
	paths := []string{
		"error", "response.error",
		"error.input_offset", "error.input_range", "error.offset", "error.details.input_offset", "error.details.input_range", "error.details.offset", "error.metadata.input_offset", "error.metadata.input_range", "error.metadata.offset",
		"response.error.input_offset", "response.error.input_range", "response.error.offset", "response.error.details.input_offset", "response.error.details.input_range", "response.error.details.offset", "response.error.metadata.input_offset", "response.error.metadata.input_range", "response.error.metadata.offset",
		"error.inputOffset", "error.inputRange", "error.match_range", "error.matchRange", "response.error.inputOffset", "response.error.inputRange", "response.error.match_range", "response.error.matchRange",
		// A few gateways lift the range alongside the error object instead of
		// nesting it. These paths are considered only after the cyber code has
		// already been identified by detectOpenAICyberPolicy.
		"input_offset", "input_range", "inputOffset", "inputRange", "match_range", "matchRange",
	}
	for _, path := range paths {
		if result := gjson.GetBytes(payload, path); result.Exists() {
			if offset, ok := parseCyberPolicyInputOffsetResult(result); ok {
				return offset, true
			}
		}
	}
	return CyberPolicyInputOffset{}, false
}

func parseCyberPolicyInputOffsetResult(result gjson.Result) (CyberPolicyInputOffset, bool) {
	if result.IsArray() {
		values := result.Array()
		if len(values) >= 2 && values[0].Type == gjson.Number && values[1].Type == gjson.Number {
			return normalizeCyberPolicyInputOffset(int(values[0].Int()), int(values[1].Int()))
		}
		if len(values) == 1 && values[0].Type == gjson.Number {
			start := int(values[0].Int())
			return normalizeCyberPolicyInputOffset(start, start+1)
		}
		return CyberPolicyInputOffset{}, false
	}
	if result.Type == gjson.Number {
		start := int(result.Int())
		return normalizeCyberPolicyInputOffset(start, start+1)
	}
	if !result.IsObject() {
		return CyberPolicyInputOffset{}, false
	}
	start := firstCyberPolicyOffsetNumber(result, "start", "start_offset", "begin", "from", "offset", "index", "position")
	end := firstCyberPolicyOffsetNumber(result, "end", "end_offset", "stop", "to")
	if start < 0 {
		return CyberPolicyInputOffset{}, false
	}
	if end < 0 {
		length := firstCyberPolicyOffsetNumber(result, "length", "count", "size")
		if length > 0 {
			end = start + length
		} else {
			end = start + 1
		}
	}
	return normalizeCyberPolicyInputOffset(start, end)
}

func firstCyberPolicyOffsetNumber(result gjson.Result, keys ...string) int {
	for _, key := range keys {
		value := result.Get(key)
		if value.Exists() && value.Type == gjson.Number {
			return int(value.Int())
		}
	}
	return -1
}

func normalizeCyberPolicyInputOffset(start, end int) (CyberPolicyInputOffset, bool) {
	if start < 0 || end <= start {
		return CyberPolicyInputOffset{}, false
	}
	return CyberPolicyInputOffset{Start: start, End: end}, true
}
