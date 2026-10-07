# Using Signalbox

Signalbox is a separate Go module. A consumer should import its Go package and load or otherwise distribute the canonical registry for the same Signalbox version:

```go
import "github.com/JiaWeiBu/SignalBox/signalbox"
```

The registry is stored as JSON under `registry/`. Use `LoadRegistry(path)` for files or `LoadRegistryFS(fs, root)` when the application embeds or otherwise provides registry files through `fs.FS`.

`LoadCanonicalRegistry()` returns Signalbox's canonical namespace assignments and any Signalbox-owned signal definitions. It does not bundle application-specific definitions. Applications should embed their own signal JSON and compose it with `LoadCanonicalNamespaces()`. `3C200001` is a format example; a registry without a definition at that code returns `ErrUnknownSignal`.

## Compose a consumer-owned catalog

Signalbox owns severity definitions, domain assignments, module assignments, schema, validation, and runtime machinery. The consumer owns signal definitions and sequence choices within its assigned modules. Embed only the consumer's signal JSON; do not copy the namespace files:

```go
//go:embed signaldefs/signals/*.json
var signalFiles embed.FS

namespaces, err := signalbox.LoadCanonicalNamespaces()
if err != nil {
	return err
}
catalog, err := signalbox.LoadSignalCatalogFS(signalFiles, "signaldefs")
if err != nil {
	return err
}
registry, err := signalbox.NewRegistryFromNamespacesAndCatalog(namespaces, catalog)
if err != nil {
	return err
}
```

Each local JSON file has the existing `{"signals":[...]}` schema. The catalog loader reads only signal files. Validation and code derivation happen during composition; subsequent event creation and lookups use in-memory maps.

## Look up a signal

Look up by stable symbolic ID when the signal identity is known by name:

```go
definition, err := registry.LookupID("myapp.local_condition")
if err != nil {
	return err
}
fmt.Printf("%s: %s\n", definition.Code, definition.Summary)
```

Or parse a code and resolve the permanent definition:

```go
code, err := signalbox.ParseSignalCode("3C200001")
if err != nil {
	return err // malformed code, unsupported severity, or reserved field
}

definition, err := registry.LookupCode(code)
switch {
case errors.Is(err, signalbox.ErrUnassignedDomain):
	// The code syntax is valid, but this domain index has no assignment.
case errors.Is(err, signalbox.ErrUnassignedModule):
	// The domain is assigned, but this module index is not assigned there.
case errors.Is(err, signalbox.ErrUnknownSignal):
	// The domain and module are assigned, but no glossary entry has this sequence.
case err != nil:
	return err
default:
	fmt.Println(definition.Summary)
}
```

`ParseSignalCode` checks only code syntax. It does not require a registry definition. `LookupCode` checks syntax, domain/module assignments, then signal registration.

## Create an event and attach runtime facts

Prefer `Registry.NewEvent` with a registered symbolic ID. It resolves the definition and uses its derived code:

```go
event, err := registry.NewEvent(
	"myapp.local_condition",
	"A local condition occurred",
	"myapp",
	nil,
)
if err != nil {
	return err
}
```

The event gets a UTC timestamp automatically. `NewSignalEventAt` accepts an explicit timestamp when reconstructing or preserving an occurrence time. Payload belongs to the event; it does not change the permanent definition or code.

## Emit a structured log

`Logger.Emit` writes one JSON object per line. It includes code, numeric severity index, message, source, timestamp, and payload:

```go
logger := signalbox.NewLogger(os.Stderr)
if err := logger.Emit(event); err != nil {
	return err
}
```

`signalbox.Emit(event)` writes JSON to stderr. Use `NewLogger` to select another writer. Signalbox does not ship logs over a network or store them.

## Wrap an event as a Go error

Not every event is an error. When an API specifically requires an `error` return, wrap the event:

```go
func run() error {
	event, err := registry.NewEvent(id, "Condition occurred", "service-main", nil)
	if err != nil {
		return err
	}
	return &signalbox.SignalError{Event: event}
}
```

The wrapper's `Error()` string contains the code and message. The complete event remains available through the `Event` field.

## Inspect or decode a code

```go
components, err := signalbox.DecodeSignalCode(signalbox.SignalCode("3C200001"))
if err != nil {
	return err
}
fmt.Printf("severity_index=%d domain=%d module=%d sequence=%d\n",
	components.Severity, components.DomainIndex, components.ModuleIndex, components.Sequence)
```

The example decodes to Major, domain 12 (Switchyard), module 2 (Chora), reserved 0, sequence 1. Use `registry.DomainByIndex` and `registry.ModuleByIndex` to resolve assigned names. `Registry.ValidateCode` checks that the domain and module indices have assignments without requiring a signal definition for the sequence.

## Consumer rules

- Do not redefine severity numbers locally.
- Do not redefine domain numbers locally.
- Do not invent module indices outside the canonical registry.
- Do not manually construct random Signal Codes or select sequences independently. Use a registered definition and its derived `Code`.
- Do not encode runtime metadata in Signal Codes. Put it in `SignalEvent.Payload`.
- Do not interpret a module number without its domain.
- Do not assume a syntactically valid code has a registered definition.

Severity, domain, and module assignments are canonical in Signalbox. Consumers maintain their own signal catalogs and follow [SPEC.md](SPEC.md) for validation and code derivation; other SDKs should use the same contract.
