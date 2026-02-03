package agent

import (
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func TestFactory_Build_SelectsToolProtocolAndAssemblesStablePrefix(t *testing.T) {
	snap := permissions.ResolveSnapshot("local", permissions.DefaultPolicy(), time.Now())

	defs, err := tool.MountWithSnapshot([]string{tool.ToolIDLs}, snap)
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	if len(defs) == 0 {
		t.Fatalf("expected defs")
	}

	f := NewFactory()
	r, err := f.Build(BuildRequest{
		Spec: AgentSpec{
			ID:           "test",
			BaseOverride: "",
			ToolIDs:      []string{tool.ToolIDLs},
			ToolProtocol: "",
		},
		Client:        &noToolsClient{},
		PolicySnapshot: snap,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if r.ToolProtocol != ToolProtocolXML {
		t.Fatalf("expected tool_protocol=xml, got %q", r.ToolProtocol)
	}
	if r.StablePrefix == "" {
		t.Fatalf("expected stable prefix")
	}
	if len(r.ToolDefs) == 0 {
		t.Fatalf("expected tool defs")
	}
}

