package fixture

import (
	"testing"
	"time"
)

func TestReset(t *testing.T) {
	s := "value"
	i := 42
	p := &i
	slice := []int{1, 2}
	mapping := map[string]int{"key": 1}
	m := Model{
		I: 1, S: "hello", B: true, F: 1.5, C: 2i, N: 5,
		Labels: Labels{"a", "b"}, Counts: Counts{"a": 1},
		Pointer: &s, Double: &p, SlicePointer: &slice, MapPointer: &mapping,
		Child: &Child{Value: 2, Data: []int{1}}, Embedded: Child{Value: 3, Data: []int{2}},
		CustomPointer: &Custom{}, Plain: struct{ Value int }{1}, Time: time.Now(),
		Array: [2]int{1, 2}, Interface: "value", Function: func() {}, Channel: make(chan int),
	}
	labels := m.Labels
	counts := m.Counts
	child := m.Child
	m.Reset()
	if m.I != 0 || m.S != "" || m.B || m.F != 0 || m.C != 0 || m.N != 0 {
		t.Fatal("primitive fields not cleared")
	}
	if m.Labels == nil || len(m.Labels) != 0 || cap(m.Labels) != cap(labels) || &m.Labels[:cap(m.Labels)][0] != &labels[0] {
		t.Fatal("slice backing storage not retained")
	}
	if m.Counts == nil || len(counts) != 0 {
		t.Fatal("map not cleared in place")
	}
	m.Counts["new"] = 1
	if counts["new"] != 1 {
		t.Fatal("map allocation changed")
	}
	if m.Pointer != &s || s != "" || m.Double != &p || p != &i || i != 0 {
		t.Fatal("pointer values not reset in place")
	}
	if m.SlicePointer != &slice || len(slice) != 0 || cap(slice) != 2 {
		t.Fatal("slice pointer reset failed")
	}
	if m.MapPointer != &mapping || mapping == nil || len(mapping) != 0 {
		t.Fatal("map pointer reset failed")
	}
	if m.Child != child || child.Value != 0 || child.Data == nil || len(child.Data) != 0 || m.Embedded.Value != 0 || len(m.Embedded.Data) != 0 {
		t.Fatal("nested generated Reset not called")
	}
	if m.Custom.Calls != 1 || m.CustomPointer.Calls != 1 {
		t.Fatal("custom Reset not called")
	}
	if m.Plain.Value != 0 || !m.Time.IsZero() || m.Array != [2]int{} || m.Interface != nil || m.Function != nil || m.Channel != nil {
		t.Fatal("remaining fields not cleared")
	}
	m.Reset()
	var zero Model
	zero.Reset()
	if zero.Pointer != nil || zero.Double != nil || zero.Child != nil || zero.Labels != nil || zero.Counts != nil {
		t.Fatal("nil values changed")
	}
	var nilModel *Model
	nilModel.Reset()
	box := Box[string]{Value: "x", Items: []string{"y"}}
	box.Reset()
	if box.Value != "" || box.Items == nil || len(box.Items) != 0 {
		t.Fatal("generic reset failed")
	}
}
