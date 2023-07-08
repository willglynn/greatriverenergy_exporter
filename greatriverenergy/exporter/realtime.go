package exporter

import (
	"context"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/willglynn/greatriverenergy_exporter/greatriverenergy"
)

type Realtime struct {
	client *greatriverenergy.Client

	conservationStatus *prometheus.Desc
	shedLikelihood     *prometheus.Desc

	shedCount        *prometheus.Desc
	shedCountResetOn *prometheus.Desc
}

func NewRealtime(rt http.RoundTripper) Realtime {
	return Realtime{
		client: greatriverenergy.NewClient(rt),

		conservationStatus: prometheus.NewDesc("greatriverenergy_conservation_gauge",
			"An indicator of electric transmission system load versus capacity. 1 = Normal, 2 = Elevated, 3 = Peak, 4 = Critical",
			nil, nil,
		),
		shedLikelihood: prometheus.NewDesc("greatriverenergy_shed_likelihood",
			"An indicator of the likelihood of using a load shedding program. 1 = Unlikely, 2 = Possible, 3 = Likely, 4 = Scheduled",
			[]string{"program", "when"}, nil,
		),

		shedCount: prometheus.NewDesc("greatriverenergy_shed_count",
			"The number of times a load shedding event occurred",
			[]string{"program"}, nil,
		),
		shedCountResetOn: prometheus.NewDesc("greatriverenergy_shed_count_reset_on",
			"The date at which the shed counts were last reset", nil, nil,
		),
	}
}

func (c Realtime) Describe(descs chan<- *prometheus.Desc) {
	descs <- c.conservationStatus
	descs <- c.shedLikelihood
	descs <- c.shedCount
	descs <- c.shedCountResetOn
}

func (c Realtime) Collect(metrics chan<- prometheus.Metric) {
	ctx := context.Background()

	if schedule, err := c.client.Schedule(ctx); err != nil {
		log.Printf("Schedule() failed: %v", err)
	} else {
		metrics <- prometheus.MustNewConstMetric(c.conservationStatus, prometheus.GaugeValue, float64(schedule.ConservationGauge))

		for when, programs := range map[string][]greatriverenergy.ProgramSchedule{
			"today":    schedule.Today,
			"next_day": schedule.NextDay,
		} {
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
}

var _ prometheus.Collector = &Realtime{}
