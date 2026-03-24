package panelassets

import "embed"

// Files stores embedded templates and static assets for single-binary distribution.
//
//go:embed templates/*.html templates/*/*.html static/* static/*/*
var Files embed.FS
