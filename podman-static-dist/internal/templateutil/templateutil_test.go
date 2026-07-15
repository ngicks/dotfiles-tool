package templateutil

import "testing"

// TestFuncDocs_MatchesFuncMap keeps FuncDocs and FuncMap in lockstep: every
// documented helper must be registered, and every registered helper must be
// documented. A drift here means the command help lists a function templates
// cannot call, or hides one they can.
func TestFuncDocs_MatchesFuncMap(t *testing.T) {
	fm := FuncMap()
	docs := FuncDocs()

	if len(fm) != len(docs) {
		t.Fatalf("FuncMap has %d entries, FuncDocs has %d", len(fm), len(docs))
	}

	documented := make(map[string]bool, len(docs))
	for _, d := range docs {
		if d.Name == "" || d.Usage == "" || d.Desc == "" {
			t.Errorf("incomplete FuncDoc: %+v", d)
		}
		if documented[d.Name] {
			t.Errorf("duplicate FuncDoc for %q", d.Name)
		}
		documented[d.Name] = true
		if _, ok := fm[d.Name]; !ok {
			t.Errorf("FuncDocs documents %q but FuncMap does not register it", d.Name)
		}
	}
	for name := range fm {
		if !documented[name] {
			t.Errorf("FuncMap registers %q but FuncDocs does not document it", name)
		}
	}
}
