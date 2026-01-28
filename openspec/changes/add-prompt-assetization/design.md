# Design: Prompt modules

## Module layout (proposal)
```
prompts/
  base_persona.md
  tools/
    bash.md
    edit.md
    write_file.md
  providers/
    anthropic.md
    openai.md
    openrouter.md
```

## Assembly
- Stable Prefix: base persona + enabled tool manuals + provider profile + tool schemas
- Volatile TurnContext: skills/plan/observer/subagent summaries（不回写 stable prefix）

## Testing
- Snapshot-like tests on assembled prompt (string contains checks)
- Guardrails: forbid known-bad patterns (e.g. `<![CDATA[`), ensure “don’t use heredoc to write files” present

