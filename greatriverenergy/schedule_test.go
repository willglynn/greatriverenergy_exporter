package greatriverenergy

import (
	"context"
	"reflect"
	"testing"
)

func TestClient_Schedule(t *testing.T) {
	c := NewClient(nil)
	schedule, err := c.Schedule(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if schedule.ConservationGauge == 0 {
		t.Error("invalid ConservationGauge")
	}

	wantPrograms := [][]string{
		{ClassCI, "C&I Interruptible Metered"},
		{ClassCI, "C&I with GenSet"},
		{ClassCI, "Interruptible Irrigation"},
		{ClassR, "Cycled Air Conditioning"},
		{ClassR, "Interruptible Water Heating"},
	}

	for key, programs := range map[string][]ProgramSchedule{
		"Today":   schedule.Today,
		"NextDay": schedule.NextDay,
	} {
		var gotPrograms [][]string
		for _, program := range programs {
			gotPrograms = append(gotPrograms, []string{
				program.Class,
				program.ProgramType,
			})
		}

		if !reflect.DeepEqual(gotPrograms, wantPrograms) {
			t.Errorf("%s programs = %+v, want %+v", key, gotPrograms, wantPrograms)
		}
	}
}
