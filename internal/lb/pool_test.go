package lb

import (
	"testing"
)

func TestNextInstanceReturnsBackendInRoundRobinOrder(t *testing.T) {
	pool := newServicePool([]string{
		"A", "B", "C",
	})

	var got []string

	for i := 0; i < 6; i++ {
		svc := pool.nextHealthyInstance()
		got = append(got, svc.Address)
	}

	want := []string{
		"A",
		"B",
		"C",
		"A",
		"B",
		"C",
	}

	for i := range want {
		if want[i] != got[i] {
			t.Fatal("unexpected round robin order of backends")
		}
	}
}
