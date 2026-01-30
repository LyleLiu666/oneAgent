package llm

import "testing"

func TestNormalizeToolArguments(t *testing.T) {
	t.Run("validJSON", func(t *testing.T) {
		in := `{"a":1}`
		out := normalizeToolArguments(in)
		if out != in {
			t.Fatalf("expected %q, got %q", in, out)
		}
	})

	t.Run("codeFenceWithLanguage", func(t *testing.T) {
		in := "```json\n{\"a\":1}\n```"
		want := `{"a":1}`
		got := normalizeToolArguments(in)
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("codeFenceWithoutLanguage", func(t *testing.T) {
		in := "```\n{\"a\":1}\n```"
		want := `{"a":1}`
		got := normalizeToolArguments(in)
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("prefixSuffixText", func(t *testing.T) {
		in := "Here is JSON: {\"a\":1} thanks"
		want := `{"a":1}`
		got := normalizeToolArguments(in)
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("quotedJSON", func(t *testing.T) {
		in := "\"{\\\"a\\\":1}\""
		want := `{"a":1}`
		got := normalizeToolArguments(in)
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("multipleJSONObjectsPrefersLast", func(t *testing.T) {
		in := "{\"a\":1}\n{\"b\":2}"
		want := `{"b":2}`
		got := normalizeToolArguments(in)
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("arrayExtraction", func(t *testing.T) {
		in := "prefix\n[1,2,3]\nsuffix"
		want := `[1,2,3]`
		got := normalizeToolArguments(in)
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("unrepairableReturnsTrimmed", func(t *testing.T) {
		in := " not json "
		want := "not json"
		got := normalizeToolArguments(in)
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})
}

