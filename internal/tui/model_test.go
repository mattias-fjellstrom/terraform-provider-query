package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattias-fjellstrom/terraform-provider-query/internal/registry"
)

// TestProvidersLoadedDedup verifies that providers arriving via providersLoadedMsg
// are deduplicated against the seenProviders set, so that a provider appearing
// in two tier responses (or repeated by accident) is not added twice.
func TestProvidersLoadedDedup(t *testing.T) {
	m := New()

	first := providersLoadedMsg{
		tier: registry.TierOfficial,
		providers: []registry.Provider{
			{Namespace: "hashicorp", Name: "aws", Tier: "official", Downloads: 100},
			{Namespace: "hashicorp", Name: "azurerm", Tier: "official", Downloads: 200},
		},
	}
	updated, _ := m.Update(first)
	m = updated.(Model)

	// Same provider arriving again (e.g. from a second tier response) must
	// not be appended twice.
	second := providersLoadedMsg{
		tier: registry.TierPartner,
		providers: []registry.Provider{
			{Namespace: "hashicorp", Name: "aws", Tier: "official", Downloads: 100},
			{Namespace: "integrations", Name: "github", Tier: "partner", Downloads: 50},
		},
	}
	updated, _ = m.Update(second)
	m = updated.(Model)

	if got := len(m.allProviders); got != 3 {
		t.Fatalf("allProviders: got %d, want 3 (dedup expected)", got)
	}
	if got := m.loadedCount; got != 3 {
		t.Errorf("loadedCount: got %d, want 3", got)
	}
}

// TestNamespaceProvidersLoadedDedup verifies that providers fetched via the
// namespace-only search path are deduplicated against providers already loaded
// from a tier fetch, and that the status message reports new vs. already-known
// counts correctly.
func TestNamespaceProvidersLoadedDedup(t *testing.T) {
	m := New()

	// Pre-load one provider as if it came from a tier fetch.
	tierMsg := providersLoadedMsg{
		tier: registry.TierPartner,
		providers: []registry.Provider{
			{Namespace: "SAP", Name: "btp", Tier: "partner", Downloads: 50},
		},
	}
	updated, _ := m.Update(tierMsg)
	m = updated.(Model)

	// Now fetch the same namespace: btp is already known, cloudfoundry is new.
	nsMsg := namespaceProvidersLoadedMsg{
		namespace: "SAP",
		providers: []registry.Provider{
			{Namespace: "SAP", Name: "btp", Tier: "partner", Downloads: 50},
			{Namespace: "SAP", Name: "cloudfoundry", Tier: "partner", Downloads: 30},
		},
	}
	updated, _ = m.Update(nsMsg)
	m = updated.(Model)

	if got := len(m.allProviders); got != 2 {
		t.Fatalf("allProviders: got %d, want 2 (1 pre-existing + 1 new)", got)
	}
	if !m.namespacesLoaded["SAP"] {
		t.Error("namespacesLoaded[SAP]: got false, want true")
	}
	if m.namespacesLoading["SAP"] {
		t.Error("namespacesLoading[SAP]: got true, want false")
	}
	if !strings.Contains(m.statusMsg, "1 new") || !strings.Contains(m.statusMsg, "1 already") {
		t.Errorf("statusMsg: got %q, want it to mention 1 new and 1 already known", m.statusMsg)
	}
}

// TestNamespaceProvidersLoadedAllNew verifies the status message when every
// returned provider is new.
func TestNamespaceProvidersLoadedAllNew(t *testing.T) {
	m := New()

	nsMsg := namespaceProvidersLoadedMsg{
		namespace: "SAP",
		providers: []registry.Provider{
			{Namespace: "SAP", Name: "btp", Tier: "partner", Downloads: 50},
			{Namespace: "SAP", Name: "cloudfoundry", Tier: "partner", Downloads: 30},
		},
	}
	updated, _ := m.Update(nsMsg)
	m = updated.(Model)

	if got := len(m.allProviders); got != 2 {
		t.Fatalf("allProviders: got %d, want 2", got)
	}
	if !strings.Contains(m.statusMsg, "loaded 2 providers") {
		t.Errorf("statusMsg: got %q, want it to report loading 2 providers", m.statusMsg)
	}
}

// TestNamespaceProvidersLoadedEmpty verifies the status message when the
// namespace returns zero providers.
func TestNamespaceProvidersLoadedEmpty(t *testing.T) {
	m := New()

	nsMsg := namespaceProvidersLoadedMsg{
		namespace: "doesnotexist",
		providers: nil,
	}
	updated, _ := m.Update(nsMsg)
	m = updated.(Model)

	if got := len(m.allProviders); got != 0 {
		t.Errorf("allProviders: got %d, want 0", got)
	}
	if !strings.Contains(m.statusMsg, "no providers found") {
		t.Errorf("statusMsg: got %q, want it to report no providers found", m.statusMsg)
	}
}

// TestEnterOnNamespaceInputDispatchesNamespaceFetch verifies that pressing
// enter while the input contains "ns/" and the browse list has no selectable
// item triggers a namespace fetch (sets namespacesLoading and returns a non-nil
// command), without entering the loading state used for versions lookups.
func TestEnterOnNamespaceInputDispatchesNamespaceFetch(t *testing.T) {
	m := New()
	m.input.SetValue("SAP/")
	// Browse list is empty by default in New(), so SelectedItem returns nil
	// and the enter handler falls through to the input-based path.

	updated, cmd := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("expected a non-nil command from namespace-fetch dispatch")
	}
	if !m.namespacesLoading["SAP"] {
		t.Errorf("namespacesLoading[SAP]: got false, want true")
	}
	if m.namespacesLoaded["SAP"] {
		t.Errorf("namespacesLoaded[SAP]: got true, want false (fetch is in flight, not done)")
	}
	if m.state != stateBrowse {
		t.Errorf("state: got %v, want stateBrowse (namespace fetch should not transition to stateLoading)", m.state)
	}
	if !strings.Contains(m.statusMsg, "loading providers in namespace") {
		t.Errorf("statusMsg: got %q, want it to report the in-flight namespace load", m.statusMsg)
	}
}

// TestEnterOnNamespaceInputIsIdempotentWhileLoading verifies that pressing
// enter again while a namespace fetch is in flight does not re-dispatch the
// fetch. (Once the load completes and providers populate the browse list,
// enter falls through to the selected-item path, which is correct.)
func TestEnterOnNamespaceInputIsIdempotentWhileLoading(t *testing.T) {
	m := New()
	m.input.SetValue("SAP/")

	// First press: dispatches the fetch.
	updated, cmd := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	m = updated.(Model)
	if cmd == nil || !m.namespacesLoading["SAP"] {
		t.Fatalf("setup: first enter should have dispatched the fetch")
	}

	// Second press while still loading: must be a no-op (no command).
	_, cmd2 := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	if cmd2 != nil {
		t.Errorf("second enter while loading: got non-nil command, want nil (idempotent)")
	}
}

// TestEnterOnNamespaceInputAfterLoadDoesNotRefetch verifies that once a
// namespace has finished loading, pressing enter on the same "ns/" input
// (with the browse list still empty, e.g. because the namespace was filtered
// out by an additional substring) does not re-fetch.
func TestEnterOnNamespaceInputAfterLoadDoesNotRefetch(t *testing.T) {
	m := New()
	m.input.SetValue("SAP/")

	// Dispatch and complete a load.
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	m = updated.(Model)
	updated, _ = m.Update(namespaceProvidersLoadedMsg{
		namespace: "SAP",
		providers: nil, // empty result so the browse list stays empty
	})
	m = updated.(Model)
	if !m.namespacesLoaded["SAP"] {
		t.Fatalf("setup: load did not mark namespace loaded")
	}

	// Press enter again. With no providers loaded into the list and the
	// namespace already in namespacesLoaded, the handler must short-circuit.
	_, cmd := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	if cmd != nil {
		t.Errorf("enter after empty namespace load: got non-nil command, want nil (already loaded)")
	}
}
