# Signalbox

Signalbox is a standalone, reusable diagnostic vocabulary and structured event library. It gives independent applications one stable way to identify and describe operational conditions, look up canonical definitions, attach runtime facts, and emit structured logs.

Signalbox is generic shared infrastructure. It does not recover services, schedule work, run tools, notify people, or manage devices. Consuming systems decide what action a signal requires.

## Concepts

- **SignalCode** is an eight-character stable identity.
- **SignalDefinition** is the permanent glossary entry for one condition.
- **SignalEvent** is one occurrence, with a message, source, timestamp, and runtime payload.
- **Severity**, **Domain**, and **Module** are canonical registry assignments.
- **DiagnosticReference** points to a capability that may help investigate a condition. Signalbox stores the reference but never executes it.

The Go reference SDK is in `signalbox/`. The language-neutral registry is in `registry/`; its JSON files are canonical and can be consumed by non-Go implementations. The initial production definitions cover Switchyard Chora. The code `3C200001` remains a format example and test fixture, not a registered glossary entry.

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

`LoadCanonicalRegistry` loads the JSON files embedded by the Go SDK, so consumers need no registry filesystem path. `Registry.NewEvent` resolves the symbolic ID first, so it cannot create an event for an unregistered definition. Glossary additions are made centrally as described in [CONTRIBUTING.md](CONTRIBUTING.md).

## Repository layout

```text
registry/
  severities.json
  domains.json
  modules/       domain-scoped module assignments
  signals/       signal definitions, stored by numeric sequence
signalbox/       Go SDK, registry loader, logger, and tests
README.md        project overview and quick start
SPEC.md          language-neutral Signalbox contract
USAGE.md         consumer guidance and Go examples
CONTRIBUTING.md  canonical registry change process
```

The required encodings, identity rules, registry assignments, and validation contract are defined in [SPEC.md](SPEC.md). Severity, domain, module, and signal assignments are canonical in this repository's registry. Consumers must use these definitions rather than maintaining local numeric copies.
