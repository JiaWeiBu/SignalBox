# Signalbox Specification v0.1

This document defines the language-neutral Signalbox contract. SDKs in any language must follow it and use the canonical registry assignments. The Go implementation is a reference, not the source of truth for assignments.

## 1. Signal Code

A Signal Code is exactly eight ASCII characters in this layout:

```text
S D M 0 E E E E
│ │ │ │ └────── 24-bit local signal sequence
│ │ │ └──────── reserved; must be index 0 in v0.1
│ │ └────────── module index within the domain
│ └──────────── domain index
└────────────── severity index
```

Positions are zero-based: severity at 0, domain at 1, module at 2, reserved at 3, and sequence at 4–7. The reserved character is always `0` in v0.1.

### Alphabet and index mapping

The alphabet, in index order, is exactly:

```text
0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_
```

| Character | Index | Character | Index | Character | Index | Character | Index |
|---|---:|---|---:|---|---:|---|---:|
| `0`–`9` | 0–9 | `A`–`Z` | 10–35 | `a`–`z` | 36–61 | `-` | 62 |
| `_` | 63 | | | | | | |

Each character encodes one integer in 0–63. Case is significant. No other character is valid.

### Sequence

The last four characters form one unsigned base-64 integer, most significant character first. The allowed numeric range is 0 through 16,777,215 (`64^4 - 1`). Sequence `1` is encoded as `0001`; the maximum sequence is `____`.

Sequence uniqueness is scoped by **domain + module**, regardless of severity. Although severity contributes to the derived Signal Code, consumers share one sequence space within each assigned module. Registry files store the integer, never a manually encoded four-character sequence.

### Severity

Severity is part of Signal Code identity. Changing a definition's severity changes its code and is a breaking identity change.

| Index | Name | Meaning | Typical response guidance |
|---:|---|---|---|
| 0 | Normal | Information, pass, or expected operation. | Log or observe. |
| 1 | Warning | Unusual condition with no actual operational impact. | Observe and continue. |
| 2 | Minor | Small defect or degraded behavior safe to leave temporarily. | Repair when convenient. |
| 3 | Major | Significant problem, though the system can continue. | Generally prioritize repair within the day. |
| 4 | Critical | The affected component cannot safely perform its normal function or is repeatedly crashing. | A higher-level system may stop, restart, or isolate it. Signalbox does none of these. |
| 5 | Emergency | Highest machine-handled severity; immediate attention is required. | Higher-level orchestration may pre-empt lower-priority work. Signalbox implements no such policy. |
| 6 | Human | Full autonomous resolution must not proceed. | Preserve safe operation and require human intervention. Signalbox does not notify the human. |

Indices 7–63 are not valid severity values in v0.1.

### Domain

Domain indices are global and canonical in Signalbox. The initial allocations are:

| Index | Code character | Domain |
|---:|:---:|---|
| 10 | `A` | QUINCUNX common/shared |
| 11 | `B` | Furnace |
| 12 | `C` | Switchyard |
| 13 | `D` | Puppeteer |
| 14 | `E` | Airlock |
| 15 | `F` | Foundry |

Indices 16–20 are reserved for QUINCUNX expansion. They have no assigned meaning and must not be used as domains. Other unassigned indices likewise have no meaning until added to the canonical registry. The registry may assign future indices to non-QUINCUNX applications.

### Module

Module indices are scoped to a domain. A module number has no meaning without its domain. For example, Switchyard module 2 is Chora; this does not assign module 2 in any other domain.

Initial modules are:

- QUINCUNX common/shared, Furnace, Puppeteer, Airlock, and Foundry: `0 = common`.
- Switchyard: `0 = common`, `1 = registry`, `2 = chora`.

No additional Switchyard modules are assigned in v0.1. A consumer must not infer a module from its numeric value alone.

### Worked format example

```text
3C200001
```

decodes to severity 3 (Major), domain index 12 (Switchyard), module index 2 (Chora), reserved 0, sequence 1. This is a format example only; it is not a registered production signal.

## 2. Definitions and events

### SignalDefinition

A SignalDefinition is the permanent glossary meaning of one condition. It includes a derived Signal Code, stable symbolic ID, severity, domain, module, numeric sequence, concise summary, optional description, and optional diagnostic references.

It must not include occurrence-specific values such as hostname, device ID, request ID, timestamp, queue position, available memory, allocation ID, retry count, agent identity, or retry delay.

### SignalEvent

A SignalEvent represents one occurrence of a SignalDefinition. It carries the Signal Code, runtime message, source, timestamp, and optional payload of runtime facts. It does not redefine the permanent meaning of the code.

### DiagnosticReference

A DiagnosticReference contains a capability ID and a short purpose. It is catalogue information that may guide an operator or another system toward diagnostics. Signalbox does not execute the capability and has no dependency on Airlock or any tool protocol.

## 3. Stable identity

The same underlying condition retains the same Signal Code regardless of which device or host produced it, when it occurred, which request caused it, how many resources were available, or how many retries happened.

Runtime metadata belongs in SignalEvent. It must never be encoded into the Signal Code. Registry maintainers must not reuse an allocated identity for a different condition.

## 4. Registry and validation

Signalbox globally owns severity definitions, domain and module assignments, the JSON schema, validation, runtime registry machinery, `SignalEvent`, `SignalError`, and the logger. Applications own application-specific signal IDs, severity choices, sequences within assigned modules, descriptions, and payload contracts. The v0.1 files use JSON so the Go SDK can load them with the standard library and other languages can consume the same portable data directly. JSON is the registry file format; the Go SDK does not introduce another authoritative copy.

Registry validation must reject:

- duplicate severity indices or IDs;
- duplicate domain indices or IDs;
- duplicate module indices or IDs within a domain;
- duplicate signal IDs;
- duplicate sequence values within a domain/module namespace, even across severities;
- duplicate derived Signal Codes;
- indices outside 0–63, severities outside 0–6, or sequences outside 0–16,777,215;
- any non-zero reserved value;
- signal references to unknown severity, domain, or module assignments;
- malformed signal IDs, empty summaries, or diagnostic references without a valid capability ID and purpose.

Code-format validation is distinct from registry lookup. A syntactically valid code may refer to an unassigned domain, an unassigned module within an assigned domain, or an unregistered sequence within assigned indices. Implementations should report those separately from malformed characters, length, severity, or reserved-field errors.

## 5. Registry composition and ownership

The Go API supports two inputs: canonical namespace assignments and an application-owned signal catalog. `LoadCanonicalNamespaces` reads only severity, domain, and module files embedded by Signalbox. `LoadSignalCatalogFS` reads signal definition files from an `fs.FS` subtree using the `{"signals":[...]}` schema. `NewRegistryFromNamespacesAndCatalog` validates and composes them into an in-memory `Registry`. Event creation and lookups use only this constructed registry; they do not read files or parse JSON.

`LoadCanonicalRegistry` loads Signalbox's canonical namespaces and any Signalbox-owned signal definitions. It does not include application-specific definitions. Applications embed and load their own catalogs, then compose them with canonical namespaces.

Signalbox defines and communicates conditions. It does not own recovery, agent reasoning, queues, scheduling, MCP execution, restart behavior, human notification, device management, remote log shipping, telemetry servers, or persistent databases. Consumers decide how to respond to an event.
