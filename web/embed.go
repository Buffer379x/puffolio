package web

import "embed"

// Dist contains the built, versioned frontend. Build with pnpm --dir web build first.
//go:embed all:dist
var Dist embed.FS
