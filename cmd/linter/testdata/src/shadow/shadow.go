package shadow

import "testing"

func panic(any) {}

type logger struct{}

func (logger) Fatal(...any) {}
func (logger) Exit(int)     {}

func calls(t *testing.T) {
	panic("user-defined function")
	log := logger{}
	log.Fatal("user-defined method")
	os := logger{}
	os.Exit(0)
	t.Fatal("testing method")
}
