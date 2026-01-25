package web

import "embed"

// Static holds the embedded frontend assets.
//
// Build pipeline should copy the frontend build output into backend/internal/web/static/.
//
//go:embed static/*
var Static embed.FS
