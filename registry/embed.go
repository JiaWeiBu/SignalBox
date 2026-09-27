// Package registry exposes the language-neutral canonical registry files as an
// embedded filesystem for consumers of the Signalbox Go SDK.
package registry

import "embed"

// FS contains the canonical registry JSON files. The JSON remains the source
// of truth for every language binding.
//
//go:embed *.json modules/*.json signals/*.json
var FS embed.FS
