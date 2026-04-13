package config

import "testing"

func TestLoad_ReadsMemorySDKFeatureFlagsFromEnv(t *testing.T) {
	t.Setenv("ONEAGENT_HOME", t.TempDir())
	t.Setenv("MEMORYSDK_ENABLE_TOOLS", "1")
	t.Setenv("MEMORYSDK_ENABLE_TURN_END_JOBS", "true")
	t.Setenv("MEMORYSDK_PRE_RECALL_POLICY", "session_only")

	cfg, err := Load(LoadOptions{})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.MemorySDKEnableTools {
		t.Fatalf("expected MEMORYSDK_ENABLE_TOOLS to be true")
	}
	if !cfg.MemorySDKEnableTurnEndJobs {
		t.Fatalf("expected MEMORYSDK_ENABLE_TURN_END_JOBS to be true")
	}
	if cfg.MemorySDKPreRecallPolicy != "session_only" {
		t.Fatalf("expected prerecall policy from env, got %q", cfg.MemorySDKPreRecallPolicy)
	}
}
