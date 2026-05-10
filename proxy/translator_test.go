package proxy

import (
	"strings"
	"testing"
)

func TestExtractOpenAIMessageTextStructured(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "text", "text": "alpha"},
		map[string]interface{}{"type": "input_text", "text": "beta"},
	}

	if got := extractOpenAIMessageText(content); got != "alphabeta" {
		t.Fatalf("expected concatenated structured text, got %q", got)
	}

	nested := map[string]interface{}{
		"content": []interface{}{map[string]interface{}{"type": "text", "text": "nested"}},
	}
	if got := extractOpenAIMessageText(nested); got != "nested" {
		t.Fatalf("expected nested content extraction, got %q", got)
	}
}

func TestOpenAIToKiroPreservesStructuredAssistantAndToolContent(t *testing.T) {
	req := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{
				Role: "system",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "system-a"},
					map[string]interface{}{"type": "text", "text": "system-b"},
				},
			},
			{Role: "user", Content: "first-question"},
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "assistant-structured"},
				},
			},
			{
				Role:       "tool",
				ToolCallID: "call_1",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "tool-result-structured"},
				},
			},
		},
	}

	payload := OpenAIToKiro(req, false)

	if len(payload.ConversationState.History) != 2 {
		t.Fatalf("expected 2 history items, got %d", len(payload.ConversationState.History))
	}

	firstHistoryUser := payload.ConversationState.History[0].UserInputMessage
	if firstHistoryUser == nil {
		t.Fatalf("expected first history item to be user message")
	}
	if !strings.Contains(firstHistoryUser.Content, "system-a") ||
		!strings.Contains(firstHistoryUser.Content, "system-b") ||
		!strings.Contains(firstHistoryUser.Content, "first-question") {
		t.Fatalf("expected merged system+user content, got %q", firstHistoryUser.Content)
	}

	historyAssistant := payload.ConversationState.History[1].AssistantResponseMessage
	if historyAssistant == nil {
		t.Fatalf("expected second history item to be assistant message")
	}
	if historyAssistant.Content != "assistant-structured" {
		t.Fatalf("expected assistant structured content to be preserved, got %q", historyAssistant.Content)
	}

	cur := payload.ConversationState.CurrentMessage.UserInputMessage
	if !strings.Contains(cur.Content, "tool-result-structured") {
		t.Fatalf("expected tool-result continuation content, got %q", cur.Content)
	}
	if cur.UserInputMessageContext == nil || len(cur.UserInputMessageContext.ToolResults) != 1 {
		t.Fatalf("expected one tool result in current context")
	}
	gotToolText := cur.UserInputMessageContext.ToolResults[0].Content[0].Text
	if gotToolText != "tool-result-structured" {
		t.Fatalf("expected structured tool result text, got %q", gotToolText)
	}
}

func TestOpenAIToKiroAssistantMapContentInHistory(t *testing.T) {
	req := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "u1"},
			{Role: "assistant", Content: map[string]interface{}{"type": "text", "text": "assistant-map"}},
			{Role: "user", Content: "u2"},
		},
	}

	payload := OpenAIToKiro(req, false)

	if len(payload.ConversationState.History) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(payload.ConversationState.History))
	}
	assistant := payload.ConversationState.History[1].AssistantResponseMessage
	if assistant == nil {
		t.Fatalf("expected second history entry to be assistant")
	}
	if assistant.Content != "assistant-map" {
		t.Fatalf("expected assistant map content preserved, got %q", assistant.Content)
	}
}

func TestOpenAIToKiroAssistantToolCallsDoNotInjectPlaceholder(t *testing.T) {
	req := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "find weather"},
			{
				Role:    "assistant",
				Content: nil,
				ToolCalls: []ToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: "get_weather", Arguments: "{}"},
				}},
			},
			{Role: "user", Content: "continue"},
		},
	}

	payload := OpenAIToKiro(req, false)
	if len(payload.ConversationState.History) < 2 {
		t.Fatalf("expected history with assistant tool call")
	}
	assistant := payload.ConversationState.History[1].AssistantResponseMessage
	if assistant == nil {
		t.Fatalf("expected assistant history entry")
	}
	if assistant.Content != "" {
		t.Fatalf("expected empty assistant content for tool-call-only turn, got %q", assistant.Content)
	}
}

func TestOpenAIConversationIDStableFromAnchor(t *testing.T) {
	baseMessages := []OpenAIMessage{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Build calculator"},
		{Role: "assistant", Content: "Sure"},
		{Role: "user", Content: "Continue"},
	}

	reqA := &OpenAIRequest{Model: "claude-sonnet-4.5", Messages: baseMessages}
	reqB := &OpenAIRequest{Model: "claude-sonnet-4.5", Messages: append(baseMessages, OpenAIMessage{Role: "assistant", Content: "Next step"})}

	payloadA := OpenAIToKiro(reqA, false)
	payloadB := OpenAIToKiro(reqB, false)

	if payloadA.ConversationState.ConversationID == "" || payloadB.ConversationState.ConversationID == "" {
		t.Fatalf("expected non-empty conversation IDs")
	}
	if payloadA.ConversationState.ConversationID != payloadB.ConversationState.ConversationID {
		t.Fatalf("expected stable conversation ID across turns, got %q vs %q", payloadA.ConversationState.ConversationID, payloadB.ConversationState.ConversationID)
	}
}

func TestClaudeConversationIDStableFromAnchor(t *testing.T) {
	reqA := &ClaudeRequest{
		Model:  "claude-sonnet-4.5",
		System: "sys",
		Messages: []ClaudeMessage{
			{Role: "user", Content: "hello"},
		},
	}
	reqB := &ClaudeRequest{
		Model:  "claude-sonnet-4.5",
		System: "sys",
		Messages: []ClaudeMessage{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "ok"},
			{Role: "user", Content: "next"},
		},
	}

	payloadA := ClaudeToKiro(reqA, false)
	payloadB := ClaudeToKiro(reqB, false)

	if payloadA.ConversationState.ConversationID == "" || payloadB.ConversationState.ConversationID == "" {
		t.Fatalf("expected non-empty conversation IDs")
	}
	if payloadA.ConversationState.ConversationID != payloadB.ConversationState.ConversationID {
		t.Fatalf("expected stable conversation ID across turns, got %q vs %q", payloadA.ConversationState.ConversationID, payloadB.ConversationState.ConversationID)
	}
}

func TestClaudeToKiroDropsClaudeCodeHarnessSystemPrompt(t *testing.T) {
	req := &ClaudeRequest{
		Model:  "claude-sonnet-4.5",
		System: "You are Claude Code, Anthropic's official CLI for Claude.\nUse /Users/yuan/.claude/projects/-Users-yuan/memory/ for memory.",
		Messages: []ClaudeMessage{
			{Role: "user", Content: "real task"},
		},
	}

	payload := ClaudeToKiro(req, false)
	content := payload.ConversationState.CurrentMessage.UserInputMessage.Content

	if strings.Contains(content, "Claude Code") || strings.Contains(content, ".claude/projects") {
		t.Fatalf("expected Claude Code harness system prompt to be dropped, got %q", content)
	}
	if content != "real task" {
		t.Fatalf("expected only user content, got %q", content)
	}
}

func TestClaudeToKiroDoesNotWrapSystemPromptAsUserPromptInjection(t *testing.T) {
	req := &ClaudeRequest{
		Model:  "claude-sonnet-4.5",
		System: "Answer in concise JSON.",
		Messages: []ClaudeMessage{
			{Role: "user", Content: "real task"},
		},
	}

	payload := ClaudeToKiro(req, false)
	content := payload.ConversationState.CurrentMessage.UserInputMessage.Content

	if strings.Contains(content, "SYSTEM PROMPT") || strings.Contains(content, "END SYSTEM PROMPT") {
		t.Fatalf("expected no system prompt wrapper, got %q", content)
	}
	if !strings.Contains(content, "Answer in concise JSON.") || !strings.Contains(content, "real task") {
		t.Fatalf("expected retained plain system instruction and user content, got %q", content)
	}
}

func TestOpenAIToKiroDropsClaudeCodeHarnessSystemPrompt(t *testing.T) {
	req := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{Role: "system", Content: "You are Claude Code, Anthropic's official CLI for Claude.\nUse /Users/yuan/.claude/projects/-Users-yuan/memory/ for memory."},
			{Role: "user", Content: "real task"},
		},
	}

	payload := OpenAIToKiro(req, false)
	content := payload.ConversationState.CurrentMessage.UserInputMessage.Content

	if strings.Contains(content, "Claude Code") || strings.Contains(content, ".claude/projects") {
		t.Fatalf("expected Claude Code harness system prompt to be dropped, got %q", content)
	}
	if content != "real task" {
		t.Fatalf("expected only user content, got %q", content)
	}
}

func TestRequestLogRedactsClaudeCodeHarnessSystemPrompt(t *testing.T) {
	claudeReq := &ClaudeRequest{
		Model:  "claude-sonnet-4.5",
		System: "You are Claude Code, Anthropic's official CLI for Claude.\nUse /Users/yuan/.claude/projects/-Users-yuan/memory/ for memory.",
		Messages: []ClaudeMessage{
			{Role: "user", Content: "real task"},
		},
	}
	claudeLog := formatClaudeRequestLogInput(claudeReq)
	if strings.Contains(claudeLog, ".claude/projects") || !strings.Contains(claudeLog, "dropped claude-code system prompt") {
		t.Fatalf("expected Claude request log to redact harness system prompt, got %q", claudeLog)
	}

	openAIReq := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{Role: "system", Content: "You are Claude Code, Anthropic's official CLI for Claude.\nUse /Users/yuan/.claude/projects/-Users-yuan/memory/ for memory."},
			{Role: "user", Content: "real task"},
		},
	}
	openAILog := formatOpenAIRequestLogInput(openAIReq)
	if strings.Contains(openAILog, ".claude/projects") || !strings.Contains(openAILog, "dropped claude-code system prompt") {
		t.Fatalf("expected OpenAI request log to redact harness system prompt, got %q", openAILog)
	}
}

func TestFilterAIRequestLogLinesRedactsLegacyClaudeCodeHarnessInput(t *testing.T) {
	content := `2026/05/11 01:02:18 [AIRequest] api=claude model=claude-opus-4.7 stream=true status=success account=abc input_tokens=1 output_tokens=1 total_tokens=2 credits=0.1 duration_ms=10 error="" input="{\"system\":\"x-anthropic-billing-header: cc_version=1; cc_entrypoint=cli;\\nYou are Claude Code, Anthropic's official CLI for Claude.\\nUse /Users/yuan/.claude/projects/-Users-yuan/memory/ for memory.\",\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" output="ok"`

	got := filterAIRequestLogLines(content, 1, "all")
	if strings.Contains(got, ".claude/projects") || strings.Contains(got, "x-anthropic-billing-header") {
		t.Fatalf("expected legacy AI request input to be redacted, got %q", got)
	}
	if !strings.Contains(got, `input="[redacted: claude-code system prompt]"`) {
		t.Fatalf("expected redacted input placeholder, got %q", got)
	}
	if !strings.Contains(got, `output="ok"`) {
		t.Fatalf("expected non-sensitive output to remain, got %q", got)
	}
}

func TestOpenAIConversationIDRandomForSyntheticAnchor(t *testing.T) {
	req := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{Role: "assistant", Content: "prefill"},
		},
	}

	payloadA := OpenAIToKiro(req, false)
	payloadB := OpenAIToKiro(req, false)

	if payloadA.ConversationState.ConversationID == payloadB.ConversationState.ConversationID {
		t.Fatalf("expected synthetic anchor to generate non-deterministic conversation IDs")
	}
}

func TestClaudeToKiroDropsLeadingAssistantHistory(t *testing.T) {
	req := &ClaudeRequest{
		Model: "claude-sonnet-4.5",
		Messages: []ClaudeMessage{
			{Role: "assistant", Content: "prefill"},
			{Role: "user", Content: "real user message"},
		},
	}

	payload := ClaudeToKiro(req, false)

	if len(payload.ConversationState.History) != 0 {
		t.Fatalf("expected leading assistant-only history to be dropped, got %d entries", len(payload.ConversationState.History))
	}

	if strings.Contains(payload.ConversationState.CurrentMessage.UserInputMessage.Content, "Begin conversation") {
		t.Fatalf("unexpected synthetic Begin conversation injection in current content: %q", payload.ConversationState.CurrentMessage.UserInputMessage.Content)
	}
}

func TestToolResultsContinuationIncludesInstructionPrefix(t *testing.T) {
	req := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "find data"},
			{Role: "assistant", ToolCalls: []ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: "fetch", Arguments: "{}"},
			}}},
			{Role: "tool", ToolCallID: "call_1", Content: "result-1"},
		},
	}

	payload := OpenAIToKiro(req, false)
	content := payload.ConversationState.CurrentMessage.UserInputMessage.Content

	if !strings.Contains(content, toolResultsContinuationPrefix) {
		t.Fatalf("expected tool continuation prefix, got %q", content)
	}
	if !strings.Contains(content, "result-1") {
		t.Fatalf("expected tool result text in continuation content, got %q", content)
	}
}
