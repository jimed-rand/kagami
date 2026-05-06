package kagamiembed

import "embed"

// CalamaresFS contains the built-in Calamares configuration tree shipped with
// kagami. It is used as a fallback when references/ is not present on disk.
//
//go:embed references/calamares/**
var CalamaresFS embed.FS
