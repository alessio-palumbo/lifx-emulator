package app

import "testing"

func TestProductsSortDuplicateNamesByAscendingPID(t *testing.T) {
	products := New().Products()
	duplicates := 0
	for i := 1; i < len(products); i++ {
		previous, current := products[i-1], products[i]
		if previous.Name > current.Name {
			t.Fatalf("product names out of order: %q before %q", previous.Name, current.Name)
		}
		if previous.Name == current.Name {
			duplicates++
			if previous.ID >= current.ID {
				t.Fatalf("duplicate name %q: PID %d before %d", current.Name, previous.ID, current.ID)
			}
		}
	}
	if duplicates == 0 {
		t.Fatal("registry contained no duplicate names to exercise ordering")
	}
}
