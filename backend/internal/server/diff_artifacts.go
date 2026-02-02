package server

import (
	"context"

	"github.com/liu_y/oneAgent/backend/internal/gitutil"
)

type DiffArtifacts = gitutil.DiffArtifacts

func generateDiffArtifacts(ctx context.Context, workspaceRoot, findingsPath, outDir string) (DiffArtifacts, error) {
	return gitutil.GenerateDiffArtifacts(ctx, workspaceRoot, findingsPath, outDir)
}
