package xmlprotocol

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/agentsdk"
)

func TestStripThinking_DoesNotStripThinkingTagsInsideInlineCode(t *testing.T) {
	input := "The fix strips leaked `<thinking>` tags from messages."
	got := StripThinking(input)
	if got != input {
		t.Fatalf("expected StripThinking to preserve inline code, got %q", got)
	}
}

func TestStripThinking_DoesNotStripThinkingTagsInsideFencedCodeBlocks(t *testing.T) {
	input := "Example:\n  ````\n<thinking>code example</thinking>\n  ````\nDone."
	got := StripThinking(input)
	if got != input {
		t.Fatalf("expected StripThinking to preserve fenced code blocks, got %q", got)
	}
}

func TestStripThinking_StripsThinkingTagsOutsideCode(t *testing.T) {
	input := "Hello <thinking>internal thought</thinking> world"
	got := StripThinking(input)
	if strings.Contains(got, "internal thought") {
		t.Fatalf("expected StripThinking to remove thinking content, got %q", got)
	}
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "world") {
		t.Fatalf("expected StripThinking to keep non-thinking text, got %q", got)
	}
}

func TestExtractLatestToolData_IgnoresToolDataInsideInlineCode(t *testing.T) {
	input := "Example: `<tool_data><call><tool_name>bash</tool_name></call></tool_data>`"
	_, ok := ExtractLatestToolData(input)
	if ok {
		t.Fatalf("expected tool_data inside inline code to be ignored")
	}
}

func TestExtractLatestToolData_IgnoresToolDataInsideFencedCodeBlocks(t *testing.T) {
	input := "Example:\n```xml\n<tool_data><call><tool_name>bash</tool_name></call></tool_data>\n```\nDone."
	_, ok := ExtractLatestToolData(input)
	if ok {
		t.Fatalf("expected tool_data inside fenced code block to be ignored")
	}
}

func TestStreamFilter_DoesNotStripThinkingOrToolDataInsideCode(t *testing.T) {
	filter := &streamFilter{}

	// Inline code: <thinking> should be preserved.
	inline := "The fix strips leaked `<thinking>` tags from messages."
	gotInline := filter.Feed(inline) + filter.Flush()
	if gotInline != inline {
		t.Fatalf("expected inline code to be preserved, got %q", gotInline)
	}

	// Fenced code: <tool_data> should be preserved.
	filter = &streamFilter{}
	fenced := "Example:\n```xml\n<tool_data><call><tool_name>bash</tool_name></call></tool_data>\n```\nDone."
	gotFenced := filter.Feed(fenced) + filter.Flush()
	if gotFenced != fenced {
		t.Fatalf("expected fenced code to be preserved, got %q", gotFenced)
	}
}

func TestRunLoop_DoesNotExecuteToolDataInsideCodeFence(t *testing.T) {
	client := &scriptedClient{
		responses: []string{
			"Example:\n```xml\n<tool_data><call><tool_name>bash</tool_name><command>echo hi</command></call></tool_data>\n```\nDone.",
		},
	}

	executed := 0
	combined, err := RunLoop(context.Background(), RunLoopInput{
		Client:   client,
		Messages: []agentsdk.Message{{Role: "user", Content: "show example"}},
		Executor: funcExecutor(func(context.Context, ToolCall) (any, error) {
			executed++
			return nil, errors.New("should not execute")
		}),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if executed != 0 {
		t.Fatalf("expected no tool execution, got %d", executed)
	}
	if client.index != 1 {
		t.Fatalf("expected 1 llm call, got %d", client.index)
	}
	if !strings.Contains(combined, "<tool_data>") {
		t.Fatalf("expected combined output to contain the code example, got %q", combined)
	}
	if !strings.Contains(combined, "Done.") {
		t.Fatalf("expected combined output to contain final text, got %q", combined)
	}
}

