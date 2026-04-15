package llm

import "testing"

func TestNewClientForProvider_OpenAICodexUsesResponsesClient(t *testing.T) {
	client, err := NewClientForProvider(ProviderConfig{
		ProviderType: ProviderTypeOpenAI,
		Endpoint:     "http://example.com/v1",
		APIKey:       "sk-test",
		Model:        "gpt-5.3-codex",
	})
	if err != nil {
		t.Fatalf("NewClientForProvider: %v", err)
	}
	if _, ok := client.(*OpenAIResponsesClient); !ok {
		t.Fatalf("expected *OpenAIResponsesClient, got %T", client)
	}
}

func TestNewClientForProvider_CodexProviderUsesResponsesClient(t *testing.T) {
	client, err := NewClientForProvider(ProviderConfig{
		ProviderType: ProviderTypeCodex,
		Endpoint:     "http://example.com/v1",
		APIKey:       "sk-test",
		Model:        "gpt-5.3-codex",
	})
	if err != nil {
		t.Fatalf("NewClientForProvider: %v", err)
	}
	if _, ok := client.(*OpenAIResponsesClient); !ok {
		t.Fatalf("expected *OpenAIResponsesClient, got %T", client)
	}
}

func TestNewClientForProvider_OpenAINonCodexKeepsChatCompletionsClient(t *testing.T) {
	client, err := NewClientForProvider(ProviderConfig{
		ProviderType: ProviderTypeOpenAI,
		Endpoint:     "http://example.com/v1",
		APIKey:       "sk-test",
		Model:        "gpt-4.1",
	})
	if err != nil {
		t.Fatalf("NewClientForProvider: %v", err)
	}
	if _, ok := client.(*OpenAIClient); !ok {
		t.Fatalf("expected *OpenAIClient, got %T", client)
	}
}
