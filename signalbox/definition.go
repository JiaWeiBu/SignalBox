package signalbox

// SeverityDefinition is a canonical human-readable severity registry entry.
type SeverityDefinition struct {
	Index           uint8  `json:"index"`
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	TypicalResponse string `json:"typical_response"`
}

// Domain is a canonical domain assignment. Its index occupies character 1 of a code.
type Domain struct {
	Index       uint8  `json:"index"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Module is a domain-scoped canonical module assignment. Its index occupies character 2.
type Module struct {
	Index       uint8  `json:"index"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	DomainID    string `json:"-"`
}

// DiagnosticReference points to a diagnostic capability; Signalbox never executes it.
type DiagnosticReference struct {
	CapabilityID string `json:"capability_id"`
	Purpose      string `json:"purpose"`
}

// SignalDefinition is the permanent glossary meaning for one registered Signal Code.
// Runtime incident details belong in SignalEvent, not here.
type SignalDefinition struct {
	Code        SignalCode            `json:"-"`
	ID          string                `json:"id"`
	Severity    Severity              `json:"severity"`
	Domain      Domain                `json:"domain"`
	Module      Module                `json:"module"`
	Sequence    uint32                `json:"sequence"`
	Summary     string                `json:"summary"`
	Description string                `json:"description,omitempty"`
	Diagnostics []DiagnosticReference `json:"diagnostics,omitempty"`
}

// SignalRecord is the portable registry representation of a SignalDefinition.
// It uses symbolic domain/module IDs and a numeric sequence; the final code is derived.
type SignalRecord struct {
	ID          string                `json:"id"`
	Severity    Severity              `json:"severity"`
	DomainID    string                `json:"domain_id"`
	ModuleID    string                `json:"module_id"`
	Sequence    uint32                `json:"sequence"`
	Reserved    uint8                 `json:"reserved,omitempty"`
	Summary     string                `json:"summary"`
	Description string                `json:"description,omitempty"`
	Diagnostics []DiagnosticReference `json:"diagnostics,omitempty"`
}

// ModuleSet groups module assignments under their owning domain in the registry files.
type ModuleSet struct {
	DomainID string   `json:"domain_id"`
	Modules  []Module `json:"modules"`
}

// RegistryData is the decoded, language-neutral registry source used to build a Registry.
type RegistryData struct {
	Severities []SeverityDefinition `json:"severities"`
	Domains    []Domain             `json:"domains"`
	Modules    []ModuleSet          `json:"module_sets"`
	Signals    []SignalRecord       `json:"signals"`
}

// NamespaceRegistry contains the canonical severity, domain, and module
// assignments used to validate consumer-owned signal catalogs.
type NamespaceRegistry struct {
	Severities []SeverityDefinition `json:"severities"`
	Domains    []Domain             `json:"domains"`
	Modules    []ModuleSet          `json:"module_sets"`
}

// SignalCatalog contains signal definitions owned by one consumer.
type SignalCatalog struct {
	Signals []SignalRecord `json:"signals"`
}
