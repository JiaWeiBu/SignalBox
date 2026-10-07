package signalbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	canonicalregistry "github.com/JiaWeiBu/SignalBox/registry"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	ErrInvalidRegistry   = errors.New("signalbox: invalid registry")
	ErrInvalidDefinition = errors.New("signalbox: invalid registry definition")
	ErrDuplicateIndex    = errors.New("signalbox: duplicate registry index")
	ErrDuplicateID       = errors.New("signalbox: duplicate registry ID")
	ErrDuplicateSequence = errors.New("signalbox: duplicate signal sequence")
	ErrDuplicateCode     = errors.New("signalbox: duplicate Signal Code")
	ErrMissingSeverity   = errors.New("signalbox: required severity is missing")
)

var identifierPattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`)

// ValidationErrors contains every registry validation problem found in one pass.
type ValidationErrors []error

func (errs ValidationErrors) Error() string {
	parts := make([]string, len(errs))
	for i, err := range errs {
		parts[i] = err.Error()
	}
	return "signalbox: registry validation failed: " + strings.Join(parts, "; ")
}

// Unwrap supports errors.Is and errors.As for each reported registry problem.
func (errs ValidationErrors) Unwrap() []error { return []error(errs) }

type moduleKey struct {
	domainIndex uint8
	moduleIndex uint8
}

type moduleIDKey struct {
	domainID string
	moduleID string
}

type sequenceKey struct {
	domainIndex uint8
	moduleIndex uint8
	sequence    uint32
}

// Registry is an immutable validated view of the canonical registry data.
type Registry struct {
	severities    map[Severity]SeverityDefinition
	domains       map[uint8]Domain
	domainsByID   map[string]Domain
	modules       map[moduleKey]Module
	modulesByID   map[moduleIDKey]Module
	signalsByCode map[SignalCode]SignalDefinition
	signalsByID   map[string]SignalDefinition
}

// LoadRegistry reads severities.json, domains.json, module JSON files, and signal JSON
// files from dir, validates all assignments, and returns an immutable registry.
func LoadRegistry(dir string) (*Registry, error) {
	var data RegistryData
	if err := readJSONFile(filepath.Join(dir, "severities.json"), &data.Severities); err != nil {
		return nil, err
	}
	if err := readJSONFile(filepath.Join(dir, "domains.json"), &data.Domains); err != nil {
		return nil, err
	}
	moduleFiles, err := jsonFiles(filepath.Join(dir, "modules"))
	if err != nil {
		return nil, err
	}
	for _, file := range moduleFiles {
		var moduleSet ModuleSet
		if err := readJSONFile(file, &moduleSet); err != nil {
			return nil, err
		}
		data.Modules = append(data.Modules, moduleSet)
	}
	signalFiles, err := jsonFiles(filepath.Join(dir, "signals"))
	if err != nil {
		return nil, err
	}
	for _, file := range signalFiles {
		var collection struct {
			Signals []SignalRecord `json:"signals"`
		}
		if err := readJSONFile(file, &collection); err != nil {
			return nil, err
		}
		data.Signals = append(data.Signals, collection.Signals...)
	}
	return NewRegistry(data)
}

// LoadCanonicalRegistry loads the canonical registry JSON shipped with the
// Signalbox module. Consumers do not need to locate registry files on disk.
func LoadCanonicalRegistry() (*Registry, error) {
	return LoadRegistryFS(canonicalregistry.FS, ".")
}

// LoadCanonicalNamespaces loads only the canonical severity, domain, and
// module assignments embedded by Signalbox. It does not load signal files.
func LoadCanonicalNamespaces() (NamespaceRegistry, error) {
	return LoadNamespacesFS(canonicalregistry.FS, ".")
}

// LoadNamespacesFS reads canonical namespace assignments from an fs.FS subtree.
func LoadNamespacesFS(filesystem fs.FS, root string) (NamespaceRegistry, error) {
	var namespaces NamespaceRegistry
	read := func(name string, target any) error {
		file := path.Join(root, name)
		contents, err := fs.ReadFile(filesystem, file)
		if err != nil {
			return fmt.Errorf("signalbox: read registry file %q: %w", file, err)
		}
		if err := decodeJSON(contents, target); err != nil {
			return fmt.Errorf("signalbox: decode registry file %q: %w", file, err)
		}
		return nil
	}
	if err := read("severities.json", &namespaces.Severities); err != nil {
		return NamespaceRegistry{}, err
	}
	if err := read("domains.json", &namespaces.Domains); err != nil {
		return NamespaceRegistry{}, err
	}
	dir := path.Join(root, "modules")
	entries, err := fs.ReadDir(filesystem, dir)
	if err != nil {
		return NamespaceRegistry{}, fmt.Errorf("signalbox: list registry directory %q: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".json" {
			continue
		}
		var moduleSet ModuleSet
		if err := read(path.Join("modules", entry.Name()), &moduleSet); err != nil {
			return NamespaceRegistry{}, err
		}
		namespaces.Modules = append(namespaces.Modules, moduleSet)
	}
	return namespaces, nil
}

// LoadSignalCatalogFS reads consumer-owned signal files from the signals
// directory beneath root. Each JSON file uses the {"signals":[...]} schema.
func LoadSignalCatalogFS(filesystem fs.FS, root string) (SignalCatalog, error) {
	var catalog SignalCatalog
	dir := path.Join(root, "signals")
	entries, err := fs.ReadDir(filesystem, dir)
	if err != nil {
		return SignalCatalog{}, fmt.Errorf("signalbox: list registry directory %q: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".json" {
			continue
		}
		file := path.Join(dir, entry.Name())
		contents, err := fs.ReadFile(filesystem, file)
		if err != nil {
			return SignalCatalog{}, fmt.Errorf("signalbox: read registry file %q: %w", file, err)
		}
		var collection struct {
			Signals []SignalRecord `json:"signals"`
		}
		if err := decodeJSON(contents, &collection); err != nil {
			return SignalCatalog{}, fmt.Errorf("signalbox: decode registry file %q: %w", file, err)
		}
		catalog.Signals = append(catalog.Signals, collection.Signals...)
	}
	return catalog, nil
}

// NewRegistryFromNamespacesAndCatalog validates and composes canonical
// assignments with a consumer-owned signal catalog.
func NewRegistryFromNamespacesAndCatalog(namespaces NamespaceRegistry, catalog SignalCatalog) (*Registry, error) {
	return NewRegistry(RegistryData{
		Severities: namespaces.Severities,
		Domains:    namespaces.Domains,
		Modules:    namespaces.Modules,
		Signals:    catalog.Signals,
	})
}

// LoadRegistryFS loads and validates a registry from an fs.FS subtree.
func LoadRegistryFS(filesystem fs.FS, root string) (*Registry, error) {
	data, err := RegistryDataFromFS(filesystem, root)
	if err != nil {
		return nil, err
	}
	return NewRegistry(data)
}

func readJSONFile(path string, target any) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("signalbox: read registry file %q: %w", path, err)
	}
	if err := decodeJSON(contents, target); err != nil {
		return fmt.Errorf("signalbox: decode registry file %q: %w", path, err)
	}
	return nil
}

func decodeJSON(contents []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("signalbox: registry file contains multiple JSON values")
		}
		return err
	}
	return nil
}

func jsonFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("signalbox: list registry directory %q: %w", dir, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

// NewRegistry validates data before constructing its lookup tables.
func NewRegistry(data RegistryData) (*Registry, error) {
	if err := ValidateRegistry(data); err != nil {
		return nil, err
	}
	r := &Registry{
		severities:    make(map[Severity]SeverityDefinition, len(data.Severities)),
		domains:       make(map[uint8]Domain, len(data.Domains)),
		domainsByID:   make(map[string]Domain, len(data.Domains)),
		modules:       make(map[moduleKey]Module),
		modulesByID:   make(map[moduleIDKey]Module),
		signalsByCode: make(map[SignalCode]SignalDefinition, len(data.Signals)),
		signalsByID:   make(map[string]SignalDefinition, len(data.Signals)),
	}
	for _, severity := range data.Severities {
		r.severities[Severity(severity.Index)] = severity
	}
	for _, domain := range data.Domains {
		r.domains[domain.Index] = domain
		r.domainsByID[domain.ID] = domain
	}
	for _, set := range data.Modules {
		domain := r.domainsByID[set.DomainID]
		for _, module := range set.Modules {
			module.DomainID = set.DomainID
			r.modules[moduleKey{domainIndex: domain.Index, moduleIndex: module.Index}] = module
			r.modulesByID[moduleIDKey{domainID: set.DomainID, moduleID: module.ID}] = module
		}
	}
	for _, record := range data.Signals {
		domain := r.domainsByID[record.DomainID]
		module := r.modulesByID[moduleIDKey{domainID: record.DomainID, moduleID: record.ModuleID}]
		code, _ := EncodeSignalCode(record.Severity, domain.Index, module.Index, record.Sequence)
		definition := SignalDefinition{
			Code:        code,
			ID:          record.ID,
			Severity:    record.Severity,
			Domain:      domain,
			Module:      module,
			Sequence:    record.Sequence,
			Summary:     record.Summary,
			Description: record.Description,
			Diagnostics: append([]DiagnosticReference(nil), record.Diagnostics...),
		}
		r.signalsByCode[code] = definition
		r.signalsByID[record.ID] = definition
	}
	return r, nil
}

// ValidateRegistry checks the complete registry, including cross-references and derived-code collisions.
func ValidateRegistry(data RegistryData) error {
	var problems ValidationErrors
	add := func(err error) { problems = append(problems, err) }

	severityByIndex := make(map[uint8]SeverityDefinition, len(data.Severities))
	severityByID := make(map[string]SeverityDefinition, len(data.Severities))
	for _, severity := range data.Severities {
		if severity.Index > 63 {
			add(fmt.Errorf("severity %q: %w: %d", severity.ID, ErrInvalidIndex, severity.Index))
		}
		if severity.Index > uint8(SeverityHuman) {
			add(fmt.Errorf("severity %q: %w: %d", severity.ID, ErrInvalidSeverity, severity.Index))
		}
		if !validIdentifier(severity.ID) || strings.TrimSpace(severity.Name) == "" {
			add(fmt.Errorf("severity index %d: %w: ID and name must be valid and non-empty", severity.Index, ErrInvalidDefinition))
		}
		if previous, ok := severityByIndex[severity.Index]; ok {
			add(fmt.Errorf("severity index %d is used by %q and %q: %w", severity.Index, previous.ID, severity.ID, ErrDuplicateIndex))
		} else {
			severityByIndex[severity.Index] = severity
		}
		if previous, ok := severityByID[severity.ID]; ok {
			add(fmt.Errorf("severity ID %q is used at indices %d and %d: %w", severity.ID, previous.Index, severity.Index, ErrDuplicateID))
		} else {
			severityByID[severity.ID] = severity
		}
	}
	for index := uint8(0); index <= uint8(SeverityHuman); index++ {
		if _, ok := severityByIndex[index]; !ok {
			add(fmt.Errorf("severity index %d: %w", index, ErrMissingSeverity))
		}
	}

	domainByIndex := make(map[uint8]Domain, len(data.Domains))
	domainByID := make(map[string]Domain, len(data.Domains))
	for _, domain := range data.Domains {
		if domain.Index > 63 {
			add(fmt.Errorf("domain %q: %w: %d", domain.ID, ErrInvalidIndex, domain.Index))
		}
		if !validIdentifier(domain.ID) || strings.TrimSpace(domain.Name) == "" {
			add(fmt.Errorf("domain index %d: %w: ID and name must be valid and non-empty", domain.Index, ErrInvalidDefinition))
		}
		if previous, ok := domainByIndex[domain.Index]; ok {
			add(fmt.Errorf("domain index %d is used by %q and %q: %w", domain.Index, previous.ID, domain.ID, ErrDuplicateIndex))
		} else {
			domainByIndex[domain.Index] = domain
		}
		if previous, ok := domainByID[domain.ID]; ok {
			add(fmt.Errorf("domain ID %q is used at indices %d and %d: %w", domain.ID, previous.Index, domain.Index, ErrDuplicateID))
		} else {
			domainByID[domain.ID] = domain
		}
	}

	moduleByID := make(map[moduleIDKey]Module)
	moduleByIndex := make(map[moduleKey]Module)
	seenModuleSets := make(map[string]struct{})
	for _, set := range data.Modules {
		domain, ok := domainByID[set.DomainID]
		if !ok {
			add(fmt.Errorf("module set references unknown domain %q: %w", set.DomainID, ErrInvalidDefinition))
			continue
		}
		if _, exists := seenModuleSets[set.DomainID]; exists {
			add(fmt.Errorf("module set for domain %q appears more than once: %w", set.DomainID, ErrDuplicateID))
		} else {
			seenModuleSets[set.DomainID] = struct{}{}
		}
		for _, module := range set.Modules {
			module.DomainID = set.DomainID
			if module.Index > 63 {
				add(fmt.Errorf("module %q in domain %q: %w: %d", module.ID, set.DomainID, ErrInvalidIndex, module.Index))
			}
			if !validIdentifier(module.ID) || strings.TrimSpace(module.Name) == "" {
				add(fmt.Errorf("module index %d in domain %q: %w: ID and name must be valid and non-empty", module.Index, set.DomainID, ErrInvalidDefinition))
			}
			indexKey := moduleKey{domainIndex: domain.Index, moduleIndex: module.Index}
			idKey := moduleIDKey{domainID: set.DomainID, moduleID: module.ID}
			if previous, exists := moduleByIndex[indexKey]; exists {
				add(fmt.Errorf("module index %d in domain %q is used by %q and %q: %w", module.Index, set.DomainID, previous.ID, module.ID, ErrDuplicateIndex))
			} else {
				moduleByIndex[indexKey] = module
			}
			if previous, exists := moduleByID[idKey]; exists {
				add(fmt.Errorf("module ID %q in domain %q is used at indices %d and %d: %w", module.ID, set.DomainID, previous.Index, module.Index, ErrDuplicateID))
			} else {
				moduleByID[idKey] = module
			}
		}
	}

	seenSignalIDs := make(map[string]struct{})
	seenSequences := make(map[sequenceKey]string)
	seenCodes := make(map[SignalCode]string)
	for _, signal := range data.Signals {
		if !validIdentifier(signal.ID) {
			add(fmt.Errorf("signal ID %q: %w: expected a lowercase stable identifier", signal.ID, ErrInvalidDefinition))
		}
		if _, exists := seenSignalIDs[signal.ID]; exists {
			add(fmt.Errorf("signal ID %q: %w", signal.ID, ErrDuplicateID))
		}
		seenSignalIDs[signal.ID] = struct{}{}
		if !signal.Severity.Valid() {
			add(fmt.Errorf("signal %q: %w: %d", signal.ID, ErrInvalidSeverity, signal.Severity))
		}
		if _, ok := severityByIndex[uint8(signal.Severity)]; !ok || !signal.Severity.Valid() {
			add(fmt.Errorf("signal %q references unregistered severity %d: %w", signal.ID, signal.Severity, ErrInvalidDefinition))
		}
		domain, domainOK := domainByID[signal.DomainID]
		if !domainOK {
			add(fmt.Errorf("signal %q references unknown domain %q: %w", signal.ID, signal.DomainID, ErrInvalidDefinition))
		}
		module, moduleOK := moduleByID[moduleIDKey{domainID: signal.DomainID, moduleID: signal.ModuleID}]
		if !moduleOK {
			add(fmt.Errorf("signal %q references unknown module %q in domain %q: %w", signal.ID, signal.ModuleID, signal.DomainID, ErrInvalidDefinition))
		}
		if signal.Sequence > MaxSequence {
			add(fmt.Errorf("signal %q: %w: %d", signal.ID, ErrSequenceOutOfRange, signal.Sequence))
		}
		if signal.Reserved != 0 {
			add(fmt.Errorf("signal %q has reserved value %d: %w", signal.ID, signal.Reserved, ErrReservedField))
		}
		if strings.TrimSpace(signal.Summary) == "" {
			add(fmt.Errorf("signal %q: %w: summary must be non-empty", signal.ID, ErrInvalidDefinition))
		}
		for i, diagnostic := range signal.Diagnostics {
			if !validIdentifier(diagnostic.CapabilityID) || strings.TrimSpace(diagnostic.Purpose) == "" {
				add(fmt.Errorf("signal %q diagnostic %d: %w: capability_id and purpose must be valid and non-empty", signal.ID, i, ErrInvalidDefinition))
			}
		}
		if domainOK && moduleOK && signal.Severity.Valid() && signal.Sequence <= MaxSequence {
			key := sequenceKey{domainIndex: domain.Index, moduleIndex: module.Index, sequence: signal.Sequence}
			if previousID, exists := seenSequences[key]; exists {
				add(fmt.Errorf("signals %q and %q share domain/module/sequence: %w", previousID, signal.ID, ErrDuplicateSequence))
			} else {
				seenSequences[key] = signal.ID
			}
			code, err := EncodeSignalCode(signal.Severity, domain.Index, module.Index, signal.Sequence)
			if err == nil {
				if previousID, exists := seenCodes[code]; exists {
					add(fmt.Errorf("signals %q and %q both encode as %s: %w", previousID, signal.ID, code, ErrDuplicateCode))
				} else {
					seenCodes[code] = signal.ID
				}
			}
		}
	}

	if len(problems) != 0 {
		return problems
	}
	return nil
}

func validIdentifier(value string) bool { return identifierPattern.MatchString(value) }

// LookupCode resolves a registered definition. It distinguishes malformed codes, unassigned
// domain/module indices, and syntactically valid codes without a signal definition.
func (registry *Registry) LookupCode(code SignalCode) (SignalDefinition, error) {
	components, err := DecodeSignalCode(code)
	if err != nil {
		return SignalDefinition{}, err
	}
	if registry == nil {
		return SignalDefinition{}, ErrInvalidRegistry
	}
	if _, ok := registry.domains[components.DomainIndex]; !ok {
		return SignalDefinition{}, fmt.Errorf("domain index %d: %w", components.DomainIndex, ErrUnassignedDomain)
	}
	if _, ok := registry.modules[moduleKey{domainIndex: components.DomainIndex, moduleIndex: components.ModuleIndex}]; !ok {
		return SignalDefinition{}, fmt.Errorf("module index %d in domain index %d: %w", components.ModuleIndex, components.DomainIndex, ErrUnassignedModule)
	}
	definition, ok := registry.signalsByCode[code]
	if !ok {
		return SignalDefinition{}, fmt.Errorf("%s: %w", code, ErrUnknownSignal)
	}
	return cloneDefinition(definition), nil
}

// LookupID resolves a registered definition by its stable symbolic ID.
func (registry *Registry) LookupID(id string) (SignalDefinition, error) {
	if registry == nil {
		return SignalDefinition{}, ErrInvalidRegistry
	}
	definition, ok := registry.signalsByID[id]
	if !ok {
		return SignalDefinition{}, fmt.Errorf("%q: %w", id, ErrUnknownSignal)
	}
	return cloneDefinition(definition), nil
}

// NewEvent creates an event only for a registered symbolic signal ID.
func (registry *Registry) NewEvent(id, message, source string, payload map[string]any) (SignalEvent, error) {
	definition, err := registry.LookupID(id)
	if err != nil {
		return SignalEvent{}, err
	}
	return NewSignalEvent(definition.Code, message, source, payload)
}

// ValidateCode verifies syntax and that the domain and module indices are assigned.
// It does not require a SignalDefinition for the sequence.
func (registry *Registry) ValidateCode(code SignalCode) error {
	components, err := DecodeSignalCode(code)
	if err != nil {
		return err
	}
	if registry == nil {
		return ErrInvalidRegistry
	}
	if _, ok := registry.domains[components.DomainIndex]; !ok {
		return fmt.Errorf("domain index %d: %w", components.DomainIndex, ErrUnassignedDomain)
	}
	if _, ok := registry.modules[moduleKey{domainIndex: components.DomainIndex, moduleIndex: components.ModuleIndex}]; !ok {
		return fmt.Errorf("module index %d in domain index %d: %w", components.ModuleIndex, components.DomainIndex, ErrUnassignedModule)
	}
	return nil
}

// Severity returns a registered severity definition by numeric identity.
func (registry *Registry) Severity(index Severity) (SeverityDefinition, bool) {
	if registry == nil {
		return SeverityDefinition{}, false
	}
	definition, ok := registry.severities[index]
	return definition, ok
}

// DomainByIndex returns a domain assignment by numeric index.
func (registry *Registry) DomainByIndex(index uint8) (Domain, bool) {
	if registry == nil {
		return Domain{}, false
	}
	domain, ok := registry.domains[index]
	return domain, ok
}

// DomainByID returns a domain assignment by stable ID.
func (registry *Registry) DomainByID(id string) (Domain, bool) {
	if registry == nil {
		return Domain{}, false
	}
	domain, ok := registry.domainsByID[id]
	return domain, ok
}

// ModuleByIndex returns a module assignment scoped to a domain index.
func (registry *Registry) ModuleByIndex(domainIndex, moduleIndex uint8) (Module, bool) {
	if registry == nil {
		return Module{}, false
	}
	module, ok := registry.modules[moduleKey{domainIndex: domainIndex, moduleIndex: moduleIndex}]
	return module, ok
}

// ModuleByID returns a module assignment scoped to a domain ID.
func (registry *Registry) ModuleByID(domainID, moduleID string) (Module, bool) {
	if registry == nil {
		return Module{}, false
	}
	module, ok := registry.modulesByID[moduleIDKey{domainID: domainID, moduleID: moduleID}]
	return module, ok
}

// Signals returns all definitions in stable ID order.
func (registry *Registry) Signals() []SignalDefinition {
	if registry == nil {
		return nil
	}
	definitions := make([]SignalDefinition, 0, len(registry.signalsByID))
	for _, definition := range registry.signalsByID {
		definitions = append(definitions, cloneDefinition(definition))
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].ID < definitions[j].ID })
	return definitions
}

func cloneDefinition(definition SignalDefinition) SignalDefinition {
	definition.Diagnostics = append([]DiagnosticReference(nil), definition.Diagnostics...)
	return definition
}

// RegistryDataFromFS reads a registry from an fs.FS subtree. It is useful to load
// registries embedded by a consumer without depending on local filesystem paths.
func RegistryDataFromFS(filesystem fs.FS, root string) (RegistryData, error) {
	var data RegistryData
	read := func(path string, target any) error {
		contents, err := fs.ReadFile(filesystem, path)
		if err != nil {
			return fmt.Errorf("signalbox: read registry file %q: %w", path, err)
		}
		if err := decodeJSON(contents, target); err != nil {
			return fmt.Errorf("signalbox: decode registry file %q: %w", path, err)
		}
		return nil
	}
	if err := read(path.Join(root, "severities.json"), &data.Severities); err != nil {
		return RegistryData{}, err
	}
	if err := read(path.Join(root, "domains.json"), &data.Domains); err != nil {
		return RegistryData{}, err
	}
	for _, folder := range []struct {
		name string
		load func(string) error
	}{
		{name: "modules", load: func(path string) error {
			var moduleSet ModuleSet
			if err := read(path, &moduleSet); err != nil {
				return err
			}
			data.Modules = append(data.Modules, moduleSet)
			return nil
		}},
		{name: "signals", load: func(path string) error {
			var collection struct {
				Signals []SignalRecord `json:"signals"`
			}
			if err := read(path, &collection); err != nil {
				return err
			}
			data.Signals = append(data.Signals, collection.Signals...)
			return nil
		}},
	} {
		dir := path.Join(root, folder.name)
		entries, err := fs.ReadDir(filesystem, dir)
		if err != nil {
			return RegistryData{}, fmt.Errorf("signalbox: list registry directory %q: %w", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || path.Ext(entry.Name()) != ".json" {
				continue
			}
			if err := folder.load(path.Join(dir, entry.Name())); err != nil {
				return RegistryData{}, err
			}
		}
	}
	return data, nil
}
