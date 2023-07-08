package greatriverenergy

import (
	"context"
	"testing"
	"time"
)

func TestClient_ShedCounts(t *testing.T) {
	c := NewClient(nil)
	counts, err := c.ShedCounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	wantLastReset := time.Date(2014, 1, 14, 0, 0, 0, 0, tz)

	if counts.LastResetOn.Before(wantLastReset) {
		t.Errorf("expected LastResetOn > 2: %v", counts.LastResetOn)
	}

	for _, key := range []string{
		"C&I Interruptible Metered",
		"C&I with GenSet",
		"Critical peak pricing",
		"Cycled Air Conditioning",
		"Dual Fuel",
		"Dual Fuel Fall Test",
		"Dual Fuel Nick Test",
		"Interruptible Crop Driers",
		"Interruptible Irrigation",
		"Interruptible Water Heating",
		"Lake Country Power Dual Fuel",
		"Lake Country Power Interruptible Water",
		"Public Appeal for Conservation",
	} {
		if _, ok := counts.Table[key]; !ok {
			t.Errorf("Table did not contain %q", key)
		}
	}
}
