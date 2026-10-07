# Signalbox

Signalbox is a standalone, reusable diagnostic vocabulary and structured event library. It gives independent applications one stable way to identify and describe operational conditions, look up canonical definitions, attach runtime facts, and emit structured logs.

Signalbox is generic shared infrastructure. It does not recover services, schedule work, run tools, notify people, or manage devices. Consuming systems decide what action a signal requires.

## Concepts

- **SignalCode** is an eight-character stable identity.
- **SignalDefinition** is the permanent glossary entry for one condition.
- **SignalEvent** is one occurrence, with a message, source, timestamp, and runtime payload.
- **Severity**, **Domain**, and **Module** are canonical registry assignments.
- **DiagnosticReference** points to a capability that may help investigate a condition. Signalbox stores the reference but never executes it.

The Go reference SDK is in `signalbox/`. Signalbox owns global severity definitions, domain and module assignments, the schema, validation, runtime registry machinery, `SignalEvent`, `SignalError`, and the logger. Applications own their signal IDs, severity choices, sequences within assigned modules, descriptions, and payload contracts. The code `3C200001` remains a format example, not a registered glossary entry.

## Minimal Go example

```go
package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/JiaWeiBu/SignalBox/signalbox"
)

//go:embed signaldefs/signals/*.json
var signalFiles embed.FS

func main() {
	namespaces, err := signalbox.LoadCanonicalNamespaces()
	if err != nil {
		panic(err)
	}
	catalog, err := signalbox.LoadSignalCatalogFS(signalFiles, "signaldefs")
	if err != nil {
		panic(err)
	}
	registry, err := signalbox.NewRegistryFromNamespacesAndCatalog(namespaces, catalog)
	if err != nil {
		panic(err)
	}

	// Use a symbolic ID registered in this application's catalog.
	event, err := registry.NewEvent(
		"myapp.local_condition",
		"A local condition occurred",
		"myapp",
		nil,
	)
	if err != nil {
		panic(err)
	}
	if err := signalbox.NewLogger(os.Stderr).Emit(event); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
```

`LoadCanonicalRegistry` loads Signalbox's canonical namespaces and any Signalbox-owned definitions; it does not bundle application-specific signal catalogs. Applications embed their own signal JSON, load it with `LoadSignalCatalogFS`, and compose it with `LoadCanonicalNamespaces` using `NewRegistryFromNamespacesAndCatalog`. `Registry.NewEvent` resolves IDs from prebuilt in-memory maps. See [USAGE.md](USAGE.md) for details.

## Repository layout

```text
registry/
  severities.json
  domains.json
  modules/       domain-scoped module assignments
  signals/       Signalbox-owned signal definitions, if any
signalbox/       Go SDK, registry loader, logger, and tests
README.md        project overview and quick start
SPEC.md          language-neutral Signalbox contract
USAGE.md         consumer guidance and Go examples
CONTRIBUTING.md  canonical registry change process
```

The required encodings, identity rules, namespace assignments, ownership rules, and validation contract are defined in [SPEC.md](SPEC.md). Signal sequences are unique within each domain/module regardless of severity.
