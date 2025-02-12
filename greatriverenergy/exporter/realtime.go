package exporter

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/willglynn/greatriverenergy_exporter/greatriverenergy"
)

type Realtime struct {
	client *greatriverenergy.Client
	rt     http.RoundTripper

	conservationStatus *prometheus.Desc
	shedLikelihood     *prometheus.Desc
	scheduleUpdated    *prometheus.Desc

	shedCount        *prometheus.Desc
	shedCountResetOn *prometheus.Desc

	ongoingShedEvent   *prometheus.Desc
	timeUntilShedStart *prometheus.Desc
	timeUntilShedEnd   *prometheus.Desc
}

func NewRealtime(rt http.RoundTripper) Realtime {
	return Realtime{
		client: greatriverenergy.NewClient(rt),
		rt:     rt,

		conservationStatus: prometheus.NewDesc("greatriverenergy_conservation_gauge",
			"An indicator of electric transmission system load versus capacity. 1 = Normal, 2 = Elevated, 3 = Peak, 4 = Critical",
			nil, nil,
		),
		shedLikelihood: prometheus.NewDesc("greatriverenergy_shed_likelihood",
			"An indicator of the likelihood of using a load shedding program. 1 = Unlikely, 2 = Possible, 3 = Likely, 4 = Scheduled",
			[]string{"program", "when"}, nil,
		),
		scheduleUpdated: prometheus.NewDesc("greatriverenergy_scheduled_updated", "The timestamp at which the schedule was last updated", nil, nil),

		shedCount: prometheus.NewDesc("greatriverenergy_shed_count",
			"The number of times a load shedding event occurred",
			[]string{"program"}, nil,
		),
		shedCountResetOn: prometheus.NewDesc("greatriverenergy_shed_count_reset_on",
			"The date at which the shed counts were last reset", nil, nil,
		),

		ongoingShedEvent: prometheus.NewDesc("greatriverenergy_ongoing_shed_event",
			"Whether a particular load shedding program is ongoing at the present time", []string{"class", "program"}, nil,
		),
		timeUntilShedStart: prometheus.NewDesc("greatriverenergy_time_until_shed_start",
			"The number of seconds before a scheduled shed event starts", []string{"class", "program"}, nil,
		),
		timeUntilShedEnd: prometheus.NewDesc("greatriverenergy_time_until_shed_end",
			"The number of seconds before a scheduled shed event ends", []string{"class", "program"}, nil,
		),
	}
}

func (c Realtime) Describe(descs chan<- *prometheus.Desc) {
	descs <- c.conservationStatus
	descs <- c.shedLikelihood
	descs <- c.scheduleUpdated
	descs <- c.shedCount
	descs <- c.shedCountResetOn
	descs <- c.ongoingShedEvent
	descs <- c.timeUntilShedStart
	descs <- c.timeUntilShedEnd
}

func (c Realtime) Collect(metrics chan<- prometheus.Metric) {
	ctx := context.Background()

	var scheduleEvents []greatriverenergy.ProgramSchedule
	if schedule, err := c.client.Schedule(ctx); err != nil {
		log.Printf("Schedule() failed: %v", err)
	} else {
		metrics <- prometheus.MustNewConstMetric(c.conservationStatus, prometheus.GaugeValue, float64(schedule.ConservationGauge))
		metrics <- prometheus.MustNewConstMetric(c.scheduleUpdated, prometheus.GaugeValue, float64(schedule.LastUpdated.Unix()))

		for when, programs := range map[string][]greatriverenergy.ProgramSchedule{
			"today":    schedule.Today,
			"next_day": schedule.NextDay,
		} {
			scheduleEvents = append(scheduleEvents, programs...)
			for _, program := range programs {
				metrics <- prometheus.MustNewConstMetric(c.shedLikelihood, prometheus.GaugeValue, float64(program.Probability), program.ProgramType, when)
			}
		}
	}

	if shedCounts, err := c.client.ShedCounts(ctx); err != nil {
		log.Printf("ShedCounts() failed: %v", err)
	} else {
		for program, count := range shedCounts.Table {
			metrics <- prometheus.MustNewConstMetric(c.shedCount, prometheus.CounterValue, float64(count), program)
		}
		metrics <- prometheus.MustNewConstMetric(c.shedCountResetOn, prometheus.GaugeValue, float64(shedCounts.LastResetOn.Unix()))
	}

	now := time.Now()
	start := now.AddDate(0, 0, -7)
	end := time.Now().AddDate(0, 0, 2)
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

		// Merge in any scheduled events
		for _, program := range scheduleEvents {
			// Ignore any events not for this class or not scheduled
			if program.Probability != greatriverenergy.ProbabilityScheduled {
				continue
			}
			if (class == "R" && program.Class != greatriverenergy.ClassR) || (class == "CI" && program.Class != greatriverenergy.ClassCI) {
				continue
			}

			// Synthesize a record
			history.Events = append(history.Events, greatriverenergy.HistoryEvent{
				ProgramName: program.ProgramType,
				Hours:       program.ExpectedEndTime.Sub(program.ExpectedStartTime).Hours(),
				StartAt:     program.ExpectedStartTime,
				EndAt:       program.ExpectedEndTime,
			})
		}

		programOngoing := make(map[string]int)
		programStart := make(map[string]float64)
		programEnd := make(map[string]float64)

		for _, event := range history.Events {
			// Ensure this program exists in the ongoing map
			programOngoing[event.ProgramName] = programOngoing[event.ProgramName]

			// Is this event ongoing?
			if now.After(event.StartAt) && event.EndAt.After(now) {
				// Set the ongoing map to 1 and time until to 0
				programOngoing[event.ProgramName] = 1
				programStart[event.ProgramName] = 0

				// If not, will it start soon?
			} else if now.Before(event.StartAt) {
				// Determine when
				seconds := event.StartAt.Sub(now).Seconds()
				// Store it if we have no record, or if the record indicates a longer interval
				if v, ok := programStart[event.ProgramName]; !ok || v < seconds {
					programStart[event.ProgramName] = seconds
				}
			}

			// Is this event the next one that will end?
			if event.EndAt.After(now) {
				seconds := event.EndAt.Sub(now).Seconds()
				// Store it if we have no record, or if the record indicates a longer interval
				if v, ok := programEnd[event.ProgramName]; !ok || v < seconds {
					programEnd[event.ProgramName] = seconds
				}
			}
		}

		for program, ongoing := range programOngoing {
			metrics <- prometheus.NewMetricWithTimestamp(now, prometheus.MustNewConstMetric(c.ongoingShedEvent, prometheus.GaugeValue, float64(ongoing), class, program))
		}
		for program, s := range programStart {
			metrics <- prometheus.NewMetricWithTimestamp(now, prometheus.MustNewConstMetric(c.timeUntilShedStart, prometheus.GaugeValue, s, class, program))
		}
		for program, s := range programEnd {
			metrics <- prometheus.NewMetricWithTimestamp(now, prometheus.MustNewConstMetric(c.timeUntilShedEnd, prometheus.GaugeValue, s, class, program))
		}
	}
}

var _ prometheus.Collector = &Realtime{}
