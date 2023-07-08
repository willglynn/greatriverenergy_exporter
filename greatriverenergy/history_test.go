package greatriverenergy

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"
)

func ymd(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, tz)
}
func ymdhm(y, m, d, h, min int) time.Time {
	return time.Date(y, time.Month(m), d, h, min, 0, 0, tz)
}

func TestClient_History(t *testing.T) {

	for _, tc := range []struct {
		startOn, endOn time.Time
		historyType    HistoryType
		events         []HistoryEvent
	}{
		{
			ymd(2021, 7, 1), ymd(2021, 7, 4), HistoryTypeR,
			[]HistoryEvent{
				{
					ProgramName: "Interruptible Water Heating",
					Hours:       5.5,
					StartAt:     ymdhm(2021, 7, 4, 15, 0),
					EndAt:       ymdhm(2021, 7, 4, 20, 30),
				},
				{
					ProgramName: "Cycled Air Conditioning",
					Hours:       4.0,
					StartAt:     ymdhm(2021, 7, 4, 15, 30),
					EndAt:       ymdhm(2021, 7, 4, 19, 30),
				},
			},
		},
		{
			ymd(2022, 7, 1), ymd(2022, 7, 4), HistoryTypeCI,
			nil,
		},
		{
			ymd(2022, 7, 1), ymd(2022, 7, 18), HistoryTypeCI,
			[]HistoryEvent{
				{
					ProgramName: "Interruptible Irrigation",
					Hours:       4,
					StartAt:     ymdhm(2022, 7, 17, 15, 0),
					EndAt:       ymdhm(2022, 7, 17, 19, 0),
				},
				{
					ProgramName: "Interruptible Irrigation",
					Hours:       4,
					StartAt:     ymdhm(2022, 7, 18, 15, 0),
					EndAt:       ymdhm(2022, 7, 18, 19, 0),
				},
				{
					ProgramName: "C&I Interruptible Metered",
					Hours:       6,
					StartAt:     ymdhm(2022, 7, 18, 14, 0),
					EndAt:       ymdhm(2022, 7, 18, 20, 0),
				},
				{
					ProgramName: "C&I with GenSet",
					Hours:       6,
					StartAt:     ymdhm(2022, 7, 18, 14, 0),
					EndAt:       ymdhm(2022, 7, 18, 20, 0),
				},
				{
					ProgramName: "Group B C&I Interruptible Metered",
					Hours:       6,
					StartAt:     ymdhm(2022, 7, 18, 14, 0),
					EndAt:       ymdhm(2022, 7, 18, 20, 0),
				},
				{
					ProgramName: "Group B C&I with GenSet",
					Hours:       6,
					StartAt:     ymdhm(2022, 7, 18, 14, 0),
					EndAt:       ymdhm(2022, 7, 18, 20, 0),
				},
			},
		},
	} {
		name := string(tc.historyType) + " " + tc.startOn.Format("20060102") + " " + tc.endOn.Format("20060102")
		t.Run(name, func(t *testing.T) {
			c := NewClient(nil)
			history, err := c.History(context.Background(), tc.historyType, tc.startOn, tc.endOn)

			if err != nil {
				t.Fatal(err)
			}

			if history.StartOn.Format("2006-01-02") != tc.startOn.Format("2006-01-02") {
				t.Errorf("incorrect StartOn: %v", history.StartOn)
			}
			if history.EndOn.Format("2006-01-02") != tc.endOn.Format("2006-01-02") {
				t.Errorf("incorrect EndOn: %v", history.StartOn)
			}

			sort.Slice(history.Events, func(i, j int) bool {
				if history.Events[i].StartAt.Before(history.Events[j].StartAt) {
					return true
				}
				if history.Events[i].ProgramName < history.Events[j].ProgramName {
					return true
				}
				if history.Events[i].EndAt.Before(history.Events[j].EndAt) {
					return true
				}
				return false
			})

			if !reflect.DeepEqual(history.Events, tc.events) {
				t.Errorf("History() = %+v\nexpected %+v", history.Events, tc.events)
			}
		})
	}

}
