package signalbox

import (
	"errors"
	"testing"
	testingfs "testing/fstest"
)

func TestTwoLevelRegistryComposition(t *testing.T) {
	namespaces, err := LoadCanonicalNamespaces()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(namespaces.Severities); got != 7 {
		t.Errorf("canonical namespace severities = %d, want 7", got)
	}
	if got := len(namespaces.Domains); got != 6 {
		t.Errorf("canonical namespace domains = %d, want 6", got)
	}
	if got := len(namespaces.Modules); got != 6 {
		t.Errorf("canonical namespace module sets = %d, want 6", got)
	}

	filesystem := testingfs.MapFS{
		"app/signals/catalog.json": &testingfs.MapFile{Data: []byte(`{"signals":[{"id":"app.local_condition","severity":2,"domain_id":"quincunx","module_id":"common","sequence":1,"summary":"Local condition"}]}`)},
		// Catalog loading must not require or parse these namespace files.
		"app/severities.json": &testingfs.MapFile{Data: []byte(`not json`)},
	}
	catalog, err := LoadSignalCatalogFS(filesystem, "app")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Signals) != 1 || catalog.Signals[0].ID != "app.local_condition" {
		t.Fatalf("loaded catalog = %#v", catalog)
	}

	registry, err := NewRegistryFromNamespacesAndCatalog(namespaces, catalog)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := registry.LookupID("app.local_condition")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := definition.Code, SignalCode("2A000001"); got != want {
		t.Fatalf("derived code = %s, want %s", got, want)
	}
	byCode, err := registry.LookupCode(definition.Code)
	if err != nil || byCode.ID != definition.ID {
		t.Fatalf("LookupCode(%s) = %#v, %v", definition.Code, byCode, err)
	}
	event, err := registry.NewEvent(definition.ID, "runtime message", "app", nil)
	if err != nil || event.Code != definition.Code {
		t.Fatalf("NewEvent = %#v, %v", event, err)
	}
}

func TestCompositionRejectsInvalidLocalSignals(t *testing.T) {
	namespaces, err := LoadCanonicalNamespaces()
	if err != nil {
		t.Fatal(err)
	}
	base := SignalRecord{
		ID: "app.local_condition", Severity: SeverityMinor,
		DomainID: "quincunx", ModuleID: "common", Sequence: 42, Summary: "Local condition",
	}
	tests := []struct {
		name    string
		signals []SignalRecord
		want    error
	}{
		{name: "unknown domain", signals: []SignalRecord{withSignal(base, func(s *SignalRecord) { s.DomainID = "missing" })}, want: ErrInvalidDefinition},
		{name: "unknown module", signals: []SignalRecord{withSignal(base, func(s *SignalRecord) { s.ModuleID = "missing" })}, want: ErrInvalidDefinition},
		{name: "unknown severity", signals: []SignalRecord{withSignal(base, func(s *SignalRecord) { s.Severity = Severity(7) })}, want: ErrInvalidSeverity},
		{name: "duplicate symbolic ID", signals: []SignalRecord{base, withSignal(base, func(s *SignalRecord) { s.Sequence++ })}, want: ErrDuplicateID},
		{name: "duplicate domain module sequence across severities", signals: []SignalRecord{base, withSignal(base, func(s *SignalRecord) { s.ID = "app.other_condition"; s.Severity = SeverityMajor })}, want: ErrDuplicateSequence},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRegistryFromNamespacesAndCatalog(namespaces, SignalCatalog{Signals: test.signals})
			if !errors.Is(err, test.want) {
				t.Fatalf("composition error = %v, want %v", err, test.want)
			}
		})
	}
}

func withSignal(signal SignalRecord, mutate func(*SignalRecord)) SignalRecord {
	mutate(&signal)
	return signal
}

func TestCanonicalNamespacesExcludeSignalsAndCompatibilityLoaderKeepsThem(t *testing.T) {
	namespaces, err := LoadCanonicalNamespaces()
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistryFromNamespacesAndCatalog(namespaces, SignalCatalog{})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(registry.Signals()); got != 0 {
		t.Fatalf("namespace-only registry has %d signals, want 0", got)
	}

	legacy, err := LoadCanonicalRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(legacy.Signals()); got != 10 {
		t.Fatalf("compatibility loader has %d signals, want all 10 current Switchyard signals", got)
	}
}
