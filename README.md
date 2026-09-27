# Signalbox

Signalbox is a standalone, reusable diagnostic vocabulary and structured event library. It gives independent applications one stable way to identify and describe operational conditions, look up canonical definitions, attach runtime facts, and emit structured logs.

Signalbox is generic shared infrastructure. It does not recover services, schedule work, run tools, notify people, or manage devices. Consuming systems decide what action a signal requires.

## Concepts

- **SignalCode** is an eight-character stable identity.
- **SignalDefinition** is the permanent glossary entry for one condition.
- **SignalEvent** is one occurrence, with a message, source, timestamp, and runtime payload.
- **Severity**, **Domain**, and **Module** are canonical registry assignments.
- **DiagnosticReference** points to a capability that may help investigate a condition. Signalbox stores the reference but never executes it.

The Go reference SDK is in `signalbox/`. The language-neutral registry is in `registry/`; its JSON files are canonical and can be consumed by non-Go implementations. No production signal definition has been allocated yet. The code `3C200001` is a format example and a test fixture, not a registered glossary entry.

## Minimal Go example

```go
package main

import (
	"fmt"
	"os"

	"github.com/JiaWeiBu/SignalBox/signalbox"
)

func main() {
	registry, err := signalbox.LoadRegistry("registry")
	if err != nil {
		panic(err)
	}

	// Replace this illustrative ID with a signal registered in your registry.
	event, err := registry.NewEvent(
		"your-domain.module.condition",
		"The condition occurred",
		"service-main",
		map[string]any{"request_id": "req-123"},
	)
	if err != nil {
		panic(err)
	}
	if err := signalbox.NewLogger(os.Stderr).Emit(event); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
```

`Registry.NewEvent` resolves the symbolic ID first, so it cannot create an event for an unregistered definition. The starter registry deliberately has no production signal definitions; glossary additions are made centrally as described in [CONTRIBUTING.md](CONTRIBUTING.md).

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
