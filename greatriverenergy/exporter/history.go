package exporter

import (
	"context"
	"log"
	"net/http"
	"reflect"
	"sort"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/willglynn/greatriverenergy_exporter/greatriverenergy"
)

type History struct {
	rt         http.RoundTripper
	daysInPast int

	shedEvent *prometheus.Desc
}

func NewHistory(rt http.RoundTripper, daysInPast int) History {
	return History{
		rt:         rt,
		daysInPast: daysInPast,

		shedEvent: prometheus.NewDesc("greatriverenergy_shed_event",
			"A load shedding event that occurred",
			[]string{"class", "program"}, nil,
		),
	}
}

func (c History) Describe(descs chan<- *prometheus.Desc) {
	descs <- c.shedEvent
}

func (c History) Collect(metrics chan<- prometheus.Metric) {
	ctx := context.Background()

	// Calculate the date range to request
	start := time.Now().AddDate(0, 0, -c.daysInPast)
	end := time.Now().AddDate(0, 0, 1)

	for historyType, class := range map[greatriverenergy.HistoryType]string{
		greatriverenergy.HistoryTypeR:  "R",
		greatriverenergy.HistoryTypeCI: "CI",
	} {
		// Use a new client to get this history, since history retrieval is stateful
		history, err := greatriverenergy.NewClient(c.rt).History(ctx, historyType, start, end)
		if err != nil {
			log.Printf("History(%q) failed: %v", historyType, err)
			continue
		}

		// Sort by event name and then by start time
		sort.Slice(history.Events, func(i, j int) bool {
			if history.Events[i].ProgramName < history.Events[j].ProgramName {
				return true
			}
			if history.Events[i].StartAt.Before(history.Events[j].StartAt) {
				return true
			}
			return false
		})

		history.Events = deduplicateSortedEvents(history.Events)

		lastEventByProgram := make(map[string]time.Time)

		emit := func(program string, t time.Time, value float64) {
			if !lastEventByProgram[program].Before(t) {
				return
			}

			lastEventByProgram[program] = t
			metrics <- prometheus.NewMetricWithTimestamp(t, prometheus.MustNewConstMetric(c.shedEvent, prometheus.GaugeValue, value, class, program))
		}

		for i, event := range history.Events {
			// Emit a 0 before
			emit(event.ProgramName, event.StartAt.Add(-time.Minute), 0)

			// Emit a 1 for each minute the event occurred
			for t := event.StartAt; t.Before(event.EndAt); t = t.Add(time.Minute) {
				emit(event.ProgramName, t, 1)
			}

			if len(history.Events) > i+1 {
				nextEvent := history.Events[i+1]
				if nextEvent.ProgramName == event.ProgramName && !nextEvent.StartAt.After(event.EndAt) {
					// The next event is either overlapping or contiguous
					// Don't emit the trailing 0 event
					continue
				}
			}

			if event.EndAt.Add(time.Minute).Before(history.EndOn) {
				// We can be pretty confident that this was actually the end of the load management event
				// Emit a 0 after
				emit(event.ProgramName, event.EndAt.Add(time.Minute), 0)
			}
		}
	}
}

func deduplicateSortedEvents(events []greatriverenergy.HistoryEvent) []greatriverenergy.HistoryEvent {
	var out []greatriverenergy.HistoryEvent
	var lastEvent greatriverenergy.HistoryEvent
	for _, event := range events {
		if reflect.DeepEqual(event, lastEvent) {
			continue
		}

		lastEvent = event
		out = append(out, event)
	}
	return out
}

var _ prometheus.Collector = &History{}
