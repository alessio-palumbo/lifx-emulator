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

func TestProductsUseDeviceTypeClassification(t *testing.T) {
	products := New().Products()
	listed := map[int]bool{}
	for _, product := range products {
		listed[product.ID] = true
	}
	for _, pid := range []int{207, 208, 219, 220, 267, 268} {
		if !listed[pid] {
			t.Errorf("DeviceTypeHybrid PID %d is missing", pid)
		}
	}
	for _, pid := range []int{70, 71, 84, 89, 115, 116, 226} {
		if listed[pid] {
			t.Errorf("DeviceTypeSwitch PID %d should be excluded", pid)
		}
	}
}
