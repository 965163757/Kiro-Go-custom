package proxy

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeChunkBasicProgression(t *testing.T) {
	prev := ""

	if got := normalizeChunk("abc", &prev); got != "abc" {
		t.Fatalf("expected first chunk to pass through, got %q", got)
	}
	if got := normalizeChunk("abcde", &prev); got != "de" {
		t.Fatalf("expected appended delta, got %q", got)
	}
}

func TestNormalizeChunkPrefixRewindDoesNotReplay(t *testing.T) {
	prev := ""

	_ = normalizeChunk("abcde", &prev)
	if got := normalizeChunk("abc", &prev); got != "" {
		t.Fatalf("expected rewind chunk to be ignored, got %q", got)
	}
	if prev != "abcde" {
		t.Fatalf("expected previous snapshot to remain longest version, got %q", prev)
	}
	if got := normalizeChunk("abcdef", &prev); got != "f" {
		t.Fatalf("expected only unseen suffix after rewind, got %q", got)
	}
}

func TestNormalizeChunkOverlapDelta(t *testing.T) {
	prev := "hello world"

	if got := normalizeChunk("world!!!", &prev); got != "!!!" {
		t.Fatalf("expected overlap suffix delta, got %q", got)
	}
}

func TestParseEventStreamReturnsErrorForEmptyKiroResponse(t *testing.T) {
	stream := buildTestKiroEvent("meteringEvent", map[string]interface{}{"usage": 0.1})

	err := parseEventStream(bytes.NewReader(stream), &KiroStreamCallback{})
	if err == nil || !strings.Contains(err.Error(), "empty Kiro response") {
		t.Fatalf("expected empty Kiro response error, got %v", err)
	}
}

func TestParseEventStreamReturnsErrorEvent(t *testing.T) {
	stream := buildTestKiroEvent("errorEvent", map[string]interface{}{"message": "upstream failed"})

	err := parseEventStream(bytes.NewReader(stream), &KiroStreamCallback{})
	if err == nil || !strings.Contains(err.Error(), "upstream failed") {
		t.Fatalf("expected upstream error, got %v", err)
	}
}

func TestParseEventStreamAcceptsAssistantContent(t *testing.T) {
	stream := buildTestKiroEvent("assistantResponseEvent", map[string]interface{}{"content": "hello"})
	var got string

	err := parseEventStream(bytes.NewReader(stream), &KiroStreamCallback{
		OnText: func(text string, isThinking bool) {
			got += text
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected assistant content, got %q", got)
	}
}

func buildTestKiroEvent(eventType string, payload map[string]interface{}) []byte {
	header := buildTestKiroEventHeader(":event-type", eventType)
	body, _ := json.Marshal(payload)

	totalLength := 12 + len(header) + len(body) + 4
	buf := bytes.NewBuffer(make([]byte, 0, totalLength))
	_ = binary.Write(buf, binary.BigEndian, uint32(totalLength))
	_ = binary.Write(buf, binary.BigEndian, uint32(len(header)))
	_ = binary.Write(buf, binary.BigEndian, uint32(0))
	buf.Write(header)
	buf.Write(body)
	_ = binary.Write(buf, binary.BigEndian, uint32(0))
	return buf.Bytes()
}

func buildTestKiroEventHeader(name, value string) []byte {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte(byte(len(name)))
	buf.WriteString(name)
	buf.WriteByte(7)
	_ = binary.Write(buf, binary.BigEndian, uint16(len(value)))
	buf.WriteString(value)
	return buf.Bytes()
}
