package fixture

import "time"

type Number int
type Labels []string
type Counts map[string]int

type Custom struct{ Calls int }

func (c *Custom) Reset() { c.Calls++ }

// generate:reset
type Child struct {
	Value int
	Data  []int
}

// generate:reset
type Model struct {
	I             int
	S             string
	B             bool
	F             float64
	C             complex128
	N             Number
	Labels        Labels
	Counts        Counts
	Pointer       *string
	Double        **int
	SlicePointer  *[]int
	MapPointer    *map[string]int
	Child         *Child
	Embedded      Child
	Custom        Custom
	CustomPointer *Custom
	Plain         struct{ Value int }
	Time          time.Time
	Array         [2]int
	Interface     any
	Function      func()
	Channel       chan int
	_             int
}

type (
	// generate:reset
	Box[T any] struct {
		Value T
		Items []T
	}
	Ignored struct{ Value int }
)
