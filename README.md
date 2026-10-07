# Signalbox

Signalbox is a standalone, reusable diagnostic vocabulary and structured event library. It gives independent applications one stable way to identify and describe operational conditions, look up canonical definitions, attach runtime facts, and emit structured logs.

Signalbox is generic shared infrastructure. It does not recover services, schedule work, run tools, notify people, or manage devices. Consuming systems decide what action a signal requires.

## Concepts

- **SignalCode** is an eight-character stable identity.
- **SignalDefinition** is the permanent glossary entry for one condition.
- **SignalEvent** is one occurrence, with a message, source, timestamp, and runtime payload.
- **Severity**, **Domain**, and **Module** are canonical registry assignments.
- **DiagnosticReference** points to a capability that may help investigate a condition. Signalbox stores the reference but never executes it.

The Go reference SDK is in `signalbox/`. Signalbox owns severity definitions, global domain and module assignments, the schema, validation, and runtime machinery. Consumers own signal definitions and sequences within their assigned modules. Ten Switchyard definitions remain centrally stored for migration compatibility; consumers can already load their own signal catalogs and compose them with Signalbox's canonical namespaces. The code `3C200001` remains a format example and test fixture, not a registered glossary entry.

## Minimal Go example

```go
package main

import (
	"fmt"
	"os"

	"github.com/JiaWeiBu/SignalBox/signalbox"
)

func main() {
	registry, err := signalbox.LoadCanonicalRegistry()
	if err != nil {
		panic(err)
	}

	// Use a symbolic ID registered in the canonical registry.
	event, err := registry.NewEvent(
		"invalid_chora_capacity",
		"Chora capacity must be positive",
		"switchyard.chora",
		map[string]any{"capacity": -1},
	)
	if err != nil {
		panic(err)
	}
	if err := signalbox.NewLogger(os.Stderr).Emit(event); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
```

`LoadCanonicalRegistry` remains the compatibility loader for the full embedded dataset, including the ten centrally stored Switchyard definitions. New consumers can call `LoadCanonicalNamespaces`, embed only their signal JSON, load it with `LoadSignalCatalogFS`, and compose the result with `NewRegistryFromNamespacesAndCatalog`. `Registry.NewEvent` resolves the symbolic ID from prebuilt in-memory maps. See [USAGE.md](USAGE.md) for the local catalog example and [CONTRIBUTING.md](CONTRIBUTING.md) for ownership guidance.

## Repository layout

```text
registry/
  severities.json
  domains.json
  modules/       domain-scoped module assignments
  signals/       current central definitions retained for compatibility
signalbox/       Go SDK, registry loader, logger, and tests
README.md        project overview and quick start
SPEC.md          language-neutral Signalbox contract
USAGE.md         consumer guidance and Go examples
CONTRIBUTING.md  canonical registry change process
```

The required encodings, identity rules, namespace assignments, and validation contract are defined in [SPEC.md](SPEC.md). Consumers use Signalbox's shared namespace assignments and maintain their own signal catalogs.
