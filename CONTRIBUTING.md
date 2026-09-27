# Contributing to Signalbox

Signalbox is the canonical owner of severity, domain, module, and signal assignments. Registry changes affect every consumer and must be reviewed as shared API changes. Never edit Switchyard or another consumer repository to assign codes locally.

## Adding a domain

1. Check `registry/domains.json` for assigned indices.
2. Choose an unassigned index in 0–63 and a stable lowercase ID. Do not use 16–20 for a new meaning; those indices remain reserved for QUINCUNX expansion.
3. Add one domain entry with a clear display name and optional description.
4. Add a corresponding file under `registry/modules/` with its `domain_id` and the domain's initial module assignments. Module indices are scoped to that domain.
5. Run the full validation and test commands below. Update `SPEC.md` and `README.md` when the allocation is part of the shared contract.

Do not assign the same domain index or ID twice. Coordinate any change to an existing identity as a breaking change; do not silently renumber or reuse it.

## Adding a module

1. Edit only the module file for the owning domain in `registry/modules/`.
2. Choose an unassigned index from 0–63 within that domain and a stable lowercase ID.
3. Add a concise name and, where useful, a description.
4. Confirm the module index and ID are unique within that domain. The same numeric index or ID may have a separate meaning in another domain.
5. Update `SPEC.md` if the shared allocation list describes the domain's initial modules.

Switchyard v0.1 currently assigns only `0 = common`, `1 = registry`, and `2 = chora`. Do not add speculative future modules.

## Adding a signal

1. Select an already registered severity, domain, and module. Verify the module belongs to that domain.
2. Choose an unused integer sequence in 0–16,777,215 within that severity/domain/module namespace. Prefer the next available sequence; never reuse an old identity for a different condition.
3. Add a `SignalRecord` to a JSON file under `registry/signals/`. Store its stable symbolic ID, numeric severity, `domain_id`, `module_id`, numeric `sequence`, summary, and optional permanent description. Do not write the final Signal Code by hand; it is derived as SDM0EEEE.
4. Keep the definition about one permanent condition. Put timestamps, hosts, device or request IDs, resource counts, retry details, and other changing facts in consumer-created SignalEvents.
5. Add or update a test that proves lookup and code derivation, and update usage/specification text if the new definition is an example.

The severity is part of identity. Moving a signal to another severity changes its code and is a breaking identity change requiring explicit migration planning.

## Adding a diagnostic reference

Add a `diagnostics` item to the signal record with a stable lowercase `capability_id` and a non-empty `purpose`. References are hints for other systems; they do not cause Signalbox to call MCP, depend on Airlock, or execute diagnostic work.

## Validate changes

The loader rejects duplicate indices and IDs, duplicate signal sequences and derived codes, unknown references, out-of-range values, non-zero reserved values, malformed identifiers, empty summaries, and malformed diagnostic references. It distinguishes malformed codes from valid codes that use unassigned domain/module indices and from assigned indices without a registered signal.

From the repository root, run:

```text
gofmt -w signalbox/*.go
go test ./...
```

The registry is JSON and has no third-party Go dependency. Keep the registry portable for future language bindings. Do not add a database, registry server, Protobuf contract, recovery behavior, or consumer integration as part of a registry change.
