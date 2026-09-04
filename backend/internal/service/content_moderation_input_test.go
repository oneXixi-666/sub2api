package service

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// 当数组末尾不是用户消息时（典型场景：Agent 工具循环结束于 tool/assistant），
// 应直接跳过审计——不再回溯查找历史中的某条用户消息。

func TestExtractContentModerationInput_AnthropicAgentToolLoopSkipsAudit(t *testing.T) {
	body := []byte(`{
		"messages": [
			{"role":"user","content":"调用一下天气工具"},
			{"role":"assistant","content":[{"type":"tool_use","id":"tool_1","name":"weather","input":{}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool_1","content":"晴 25 度"}]}
		]
	}`)

	input := ExtractContentModerationInput(ContentModerationProtocolAnthropicMessages, body)

	require.Empty(t, input.Text)
	require.Empty(t, input.Images)
}

func TestExtractContentModerationInput_AnthropicFirstTurnExtractsUser(t *testing.T) {
	body := []byte(`{
		"messages": [
			{"role":"user","content":"Q1"}
		]
	}`)

	input := ExtractContentModerationInput(ContentModerationProtocolAnthropicMessages, body)

	require.Equal(t, "Q1", input.Text)
}

func TestExtractContentModerationInput_AnthropicMultiTurnExtractsLatestUser(t *testing.T) {
	body := []byte(`{
		"messages": [
			{"role":"user","content":"Q1"},
			{"role":"assistant","content":"A1"},
			{"role":"user","content":"Q2"}
		]
	}`)

	input := ExtractContentModerationInput(ContentModerationProtocolAnthropicMessages, body)

	require.Equal(t, "Q2", input.Text)
}

func TestExtractContentModerationInput_AnthropicStreamResendExtractsResend(t *testing.T) {
	body := []byte(`{
		"messages": [
			{"role":"user","content":"原问题"},
			{"role":"assistant","content":"部分回答……"},
			{"role":"user","content":"重发"}
		]
	}`)

	input := ExtractContentModerationInput(ContentModerationProtocolAnthropicMessages, body)

	require.Equal(t, "重发", input.Text)
}

func TestExtractContentModerationInput_OpenAIChatAgentToolLoopSkipsAudit(t *testing.T) {
	body := []byte(`{
		"messages": [
			{"role":"system","content":"sys"},
			{"role":"user","content":"列出我的订单"},
			{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"orders","arguments":"{}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"[]"}
		]
	}`)

	input := ExtractContentModerationInput(ContentModerationProtocolOpenAIChat, body)

	require.Empty(t, input.Text)
	require.Empty(t, input.Images)
}

func TestExtractContentModerationInput_OpenAIChatMultiTurnExtractsLatestUser(t *testing.T) {
	body := []byte(`{
		"messages": [
			{"role":"user","content":"Q1"},
			{"role":"assistant","content":"A1"},
			{"role":"user","content":"Q2"}
		]
	}`)

	input := ExtractContentModerationInput(ContentModerationProtocolOpenAIChat, body)

	require.Equal(t, "Q2", input.Text)
}

func TestExtractContentModerationInput_GeminiAgentToolLoopSkipsAudit(t *testing.T) {
	body := []byte(`{
		"contents": [
			{"role":"user","parts":[{"text":"查询天气"}]},
			{"role":"model","parts":[{"functionCall":{"name":"weather","args":{}}}]},
			{"role":"user","parts":[{"functionResponse":{"name":"weather","response":{"temp":25,"summary":"晴"}}}]}
		]
	}`)

	input := ExtractContentModerationInput(ContentModerationProtocolGemini, body)

	require.Empty(t, input.Text)
	require.Empty(t, input.Images)
	extracted := extractContentModerationInputs(ContentModerationProtocolGemini, body)
	require.NotEmpty(t, extracted.Segments)
	require.Equal(t, moderationRoleTool, extracted.Segments[len(extracted.Segments)-1].Role)
	require.False(t, extracted.Segments[len(extracted.Segments)-1].Enforceable)
}

func TestExtractContentModerationInput_GeminiFirstTurnExtractsUser(t *testing.T) {
	body := []byte(`{
		"contents": [
			{"role":"user","parts":[{"text":"你好"}]}
		]
	}`)

	input := ExtractContentModerationInput(ContentModerationProtocolGemini, body)

	require.Equal(t, "你好", input.Text)
}

func TestExtractContentModerationInput_GeminiMultiTurnExtractsLatestUser(t *testing.T) {
	body := []byte(`{
		"contents": [
			{"role":"user","parts":[{"text":"Q1"}]},
			{"role":"model","parts":[{"text":"A1"}]},
			{"role":"user","parts":[{"text":"Q2"}]}
		]
	}`)

	input := ExtractContentModerationInput(ContentModerationProtocolGemini, body)

	require.Equal(t, "Q2", input.Text)
}

func TestExtractContentModerationInput_ResponsesAgentToolLoopSkipsAudit(t *testing.T) {
	body := []byte(`{
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"运行测试"}]},
			{"type":"function_call","call_id":"call_1","name":"run_tests","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_1","output":"all passed"}
		]
	}`)

	input := ExtractContentModerationInput(ContentModerationProtocolOpenAIResponses, body)

	require.Empty(t, input.Text)
	require.Empty(t, input.Images)
}

func TestExtractContentModerationInput_ResponsesLastUserMessageExtracted(t *testing.T) {
	body := []byte(`{
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"first"}]},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"answer"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"latest"}]}
		]
	}`)

	input := ExtractContentModerationInput(ContentModerationProtocolOpenAIResponses, body)

	require.Equal(t, "latest", input.Text)
}

func TestExtractContentModerationInput_ResponsesLastIsAssistantSkipped(t *testing.T) {
	body := []byte(`{
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"q1"}]},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"a1"}]}
		]
	}`)

	input := ExtractContentModerationInput(ContentModerationProtocolOpenAIResponses, body)

	require.Empty(t, input.Text)
	require.Empty(t, input.Images)
}

func TestExtractContentModerationInputs_ResponsesStringRequiresAPIAudit(t *testing.T) {
	body := []byte(`{"input":"tool output with broad-policy-term and approval context"}`)

	extracted := extractContentModerationInputsWithTrust(ContentModerationProtocolOpenAIResponses, body, true)

	require.Empty(t, extracted.User.Text)
	require.Equal(t, "tool output with broad-policy-term and approval context", extracted.Audit.Text)
	require.True(t, extracted.RequiresAPI)
	require.Equal(t, moderationRoleAmbiguous, extracted.Segments[0].Role)
}

func TestExtractContentModerationInputs_ResponsesWebSocketStringRequiresAPIAudit(t *testing.T) {
	body := []byte(`{"type":"response.create","response":{"input":"websocket string input"}}`)

	extracted := extractContentModerationInputs(ContentModerationProtocolOpenAIResponsesWS, body)

	require.Empty(t, extracted.User.Text)
	require.Equal(t, "websocket string input", extracted.Audit.Text)
	require.True(t, extracted.RequiresAPI)
	require.Equal(t, moderationRoleAmbiguous, extracted.Segments[0].Role)
}

func TestExtractContentModerationInputs_ResponsesUnroledInputTextRequiresAPIAudit(t *testing.T) {
	body := []byte(`{"input":[{"type":"input_text","text":"flattened tool result"}]}`)

	extracted := extractContentModerationInputsWithTrust(ContentModerationProtocolOpenAIResponses, body, true)

	require.Empty(t, extracted.User.Text)
	require.Equal(t, "flattened tool result", extracted.Audit.Text)
	require.True(t, extracted.RequiresAPI)
}

func TestExtractContentModerationInputs_UntrustedEnvelopeCannotSkipUserHardBlock(t *testing.T) {
	body := []byte(`{"input":[{"type":"message","role":"user","content":"<environment_context>\ninternal hard-term\n</environment_context>"}]}`)

	extracted := extractContentModerationInputs(ContentModerationProtocolOpenAIResponses, body)

	require.Contains(t, extracted.User.Text, "internal hard-term")
	require.Contains(t, extracted.Segments, ModerationSegment{
		Role: moderationRoleUser, Source: "input[0].content", Text: "<environment_context>\ninternal hard-term\n</environment_context>", Enforceable: true,
	})
	require.NotContains(t, extracted.Segments, ModerationSegment{Role: moderationRoleEnvironment})
}

func TestExtractContentModerationInputs_TrustedCodexEnvelopeIsSeparated(t *testing.T) {
	body := []byte(`{"input":[{"type":"message","role":"user","content":"<codex_internal_context>\ninternal context\n</codex_internal_context>\nactual request"}]}`)

	extracted := extractContentModerationInputsWithTrust(ContentModerationProtocolOpenAIResponses, body, true)

	require.Equal(t, "actual request", extracted.User.Text)
	require.Contains(t, extracted.Segments, ModerationSegment{Role: moderationRoleEnvironment, Source: "input[0].content.envelope", Text: "<codex_internal_context>\ninternal context\n</codex_internal_context>"})
}

func TestExtractContentModerationInputs_UserOutputTextIsNotEnforceable(t *testing.T) {
	body := []byte(`{"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"safe"},{"type":"output_text","text":"hard-term"}]}]}`)

	extracted := extractContentModerationInputs(ContentModerationProtocolOpenAIResponses, body)

	require.Equal(t, "safe", extracted.User.Text)
	require.Contains(t, extracted.Segments, ModerationSegment{Role: moderationRoleUser, Source: "input[0].content[0].text", Text: "safe", Enforceable: true})
	for _, segment := range extracted.Segments {
		require.NotEqual(t, "hard-term", segment.Text)
	}
}

func TestExtractContentModerationInputs_ResponsesOutputTextParentCannotBecomeEnforceable(t *testing.T) {
	body := []byte(`{"input":[{"type":"output_text","role":"user","content":"hard-term"}]}`)

	extracted := extractContentModerationInputs(ContentModerationProtocolOpenAIResponses, body)

	for _, segment := range extracted.Segments {
		require.False(t, segment.Enforceable, "output_text parent must never be hard-blockable")
	}
}

func TestExtractContentModerationInputs_ResponsesNonUserTypesCannotBecomeEnforceable(t *testing.T) {
	for _, typ := range []string{"refusal", "tool_result", "function_call_output"} {
		t.Run(typ, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"input":[{"type":%q,"role":"user","content":"hard-term","output":"hard-term","arguments":"hard-term"}]}`, typ))

			extracted := extractContentModerationInputs(ContentModerationProtocolOpenAIResponses, body)

			for _, segment := range extracted.Segments {
				require.False(t, segment.Enforceable, "%s parent must never be hard-blockable", typ)
			}
		})
	}
}

func TestExtractContentModerationInputs_ResponsesUnsupportedUserTypeStillRequiresAPIAudit(t *testing.T) {
	for _, typ := range []string{"output_text", "refusal", "tool_result", "function_call_output"} {
		t.Run(typ, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"input":[{"type":%q,"role":"user","content":"risk text","output":"tool output"}]}`, typ))

			extracted := extractContentModerationInputs(ContentModerationProtocolOpenAIResponses, body)

			require.Empty(t, extracted.User.Text, "unsupported user item types must not become enforceable input")
			require.Contains(t, extracted.Audit.Text, "risk text")
			require.Contains(t, extracted.Audit.Text, "tool output")
			require.True(t, extracted.RequiresAPI)
			for _, segment := range extracted.Segments {
				require.False(t, segment.Enforceable, "%s item must remain non-enforceable", typ)
			}
		})
	}
}

func TestExtractContentModerationInputs_ResponsesCompleteEnvelopeIsStrippedButUserRemainderIsAudited(t *testing.T) {
	body := []byte(`{
		"input":[
			{"type":"message","role":"developer","content":[{"type":"input_text","text":"policy"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>\ntool output broad-policy-term\n</environment_context>\nplease review this request"}]}
		]
	}`)

	extracted := extractContentModerationInputsWithTrust(ContentModerationProtocolOpenAIResponses, body, true)

	require.Equal(t, "please review this request", extracted.User.Text)
	require.Equal(t, extracted.User.Text, extracted.Audit.Text)
	require.False(t, extracted.RequiresAPI)
	require.Contains(t, extracted.Segments, ModerationSegment{Role: moderationRoleUser, Source: "input[1].content[0].text", Text: "please review this request", Enforceable: true})
	require.Contains(t, extracted.Segments, ModerationSegment{Role: moderationRoleEnvironment, Source: "input[1].content[0].text.envelope", Text: "<environment_context>\ntool output broad-policy-term\n</environment_context>"})
}

func TestExtractContentModerationInputs_MarkerLiteralCannotSkipRealUserInput(t *testing.T) {
	body := []byte(`{"input":[{"type":"message","role":"user","content":"explain the literal <environment_context> marker and then do the task"}]}`)

	extracted := extractContentModerationInputs(ContentModerationProtocolOpenAIResponses, body)

	require.Equal(t, "explain the literal <environment_context> marker and then do the task", extracted.User.Text)
	require.Equal(t, extracted.User.Text, extracted.Audit.Text)
}

func TestExtractContentModerationInputs_AgentsEnvelopeRequiresCompleteHeaderAndKeepsRemainder(t *testing.T) {
	body := []byte(`{"input":[{"type":"message","role":"user","content":"# AGENTS.md instructions\n\n<INSTRUCTIONS>\ninternal policy\n</INSTRUCTIONS>\nactual user request"}]}`)

	extracted := extractContentModerationInputsWithTrust(ContentModerationProtocolOpenAIResponses, body, true)

	require.Equal(t, "actual user request", extracted.User.Text)
	require.Contains(t, extracted.Segments, ModerationSegment{
		Role: moderationRoleEnvironment, Source: "input[0].content.envelope",
		Text: "# AGENTS.md instructions\n\n<INSTRUCTIONS>\ninternal policy\n</INSTRUCTIONS>",
	})
}

func TestExtractContentModerationInputs_ResponsesContextRolesRemainSeparate(t *testing.T) {
	body := []byte(`{
		"instructions":"developer broad-policy-term",
		"input":[
			{"type":"message","role":"system","content":"system broad-policy-term"},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"assistant broad-policy-term"}]},
			{"type":"function_call_output","call_id":"call_1","output":"tool broad-policy-term"}
		]
	}`)

	extracted := extractContentModerationInputs(ContentModerationProtocolOpenAIResponses, body)

	require.Empty(t, extracted.User.Text)
	require.Empty(t, extracted.Audit.Text)
	require.Contains(t, extracted.Segments, ModerationSegment{Role: moderationRoleDeveloper, Source: "instructions", Text: "developer broad-policy-term"})
	require.Contains(t, extracted.Segments, ModerationSegment{Role: moderationRoleSystem, Source: "input[0].content", Text: "system broad-policy-term"})
	require.Contains(t, extracted.Segments, ModerationSegment{Role: moderationRoleAssistant, Source: "input[1].content[0].text", Text: "assistant broad-policy-term"})
	require.Contains(t, extracted.Segments, ModerationSegment{Role: moderationRoleTool, Source: "input[2].output", Text: "tool broad-policy-term"})
}
