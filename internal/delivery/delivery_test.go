package delivery

import "testing"

func TestNewBatchIDsAreUniqueAndValid(t *testing.T) {
	seen := make(map[string]bool)
	for range 1000 {
		id := New(nil).ID
		if !ValidKey(id) || seen[id] {
			t.Fatalf("invalid or repeated ID %q", id)
		}
		seen[id] = true
	}
	for _, key := range []string{"", "a b", "a,b", "\n", "кириллица"} {
		if ValidKey(key) {
			t.Errorf("invalid key accepted: %q", key)
		}
	}
}
