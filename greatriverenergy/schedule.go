package greatriverenergy

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Schedule struct {
	ConservationGauge ConservationStatus `json:"conservationGauge"`
	Today             []ProgramSchedule  `json:"today"`
	NextDay           []ProgramSchedule  `json:"nextDay"`
}

type ProgramSchedule struct {
	Class             string      `json:"class"`
	ProgramType       string      `json:"programType"`
	Probability       Probability `json:"probability"`
	ExpectedStartTime time.Time   `json:"expectedStartTime,omitempty"`
}

type ConservationStatus int

const (
	ConservationStatusNormalUsage = iota + 1
	ConservationStatusElevatedUsage
	ConservationStatusPeakUsage
	ConservationStatusCriticalUsage
)

func (cs ConservationStatus) String() string {
	switch cs {
	case ConservationStatusNormalUsage:
		return "Normal usage"
	case ConservationStatusElevatedUsage:
		return "Elevated usage"
	case ConservationStatusPeakUsage:
		return "Peak usage"
	case ConservationStatusCriticalUsage:
		return "Critical usage"
	default:
		return ""
	}
}

type Probability int

const (
	ProbabilityUnlikely = iota + 1
	ProbabilityPossible
	ProbabilityLikely
	ProbabilityScheduled
)

func (p Probability) String() string {
	switch p {
	case ProbabilityUnlikely:
		return "Unlikely"
	case ProbabilityPossible:
		return "Possible"
	case ProbabilityLikely:
		return "Likely"
	case ProbabilityScheduled:
		return "Scheduled"
	default:
		return ""
	}
}

const ClassCI = "CI"
const ClassR = "Residential"

func (c Client) Schedule(ctx context.Context) (*Schedule, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://lmguide.grenergy.com/Default.aspx", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected status code 200, got %v", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var conservationGauge ConservationStatus

	conservationGaugeImgSrc := doc.Find("img#ContentPlaceHolder1_Gauge_Image").AttrOr("src", "")
	switch conservationGaugeImgSrc {
	case "images/gauge1.jpg":
		conservationGauge = ConservationStatusNormalUsage
	case "images/gauge2.jpg":
		conservationGauge = ConservationStatusElevatedUsage
	case "images/gauge3.jpg":
		conservationGauge = ConservationStatusPeakUsage
	case "images/gauge4.jpg":
		conservationGauge = ConservationStatusCriticalUsage
	default:
		return nil, fmt.Errorf("scrape failure: unable to determine conservation status (src=%q)", conservationGaugeImgSrc)
	}

	// TODO: parse ymd from page
	y, m, d := time.Now().In(tz).Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, tz)
	nextDay := today.AddDate(0, 0, 1)

	todaySchedule, err := parseScheduleTable(doc.Find("#ContentPlaceHolder2_TodaySched_Table"), today)
	if err != nil {
		return nil, fmt.Errorf("scrape failure: failed to parse Today table: %v", err)
	}

	nextDaySchedule, err := parseScheduleTable(doc.Find("#ContentPlaceHolder2_NextDaySched_Table"), nextDay)
	if err != nil {
		return nil, fmt.Errorf("scrape failure: failed to parse Next Day table: %v", err)
	}

	return &Schedule{
		ConservationGauge: conservationGauge,
		Today:             todaySchedule,
		NextDay:           nextDaySchedule,
	}, nil
}

func parseScheduleTable(table *goquery.Selection, day time.Time) ([]ProgramSchedule, error) {
	var out []ProgramSchedule
	var err error
	table.Find(".BodyText_noSpaces").Each(func(_ int, tr *goquery.Selection) {
		if err != nil {
			return
		}

		cells := tr.Find("td").Map(func(_ int, td *goquery.Selection) string {
			return td.Text()
		})
		if len(cells) != 4 {
			err = fmt.Errorf("expected 4 cells, got %v", len(cells))
			return
		}

		var probability Probability
		switch cells[2] {
		case "Unlikely":
			probability = ProbabilityUnlikely
		case "Possible":
			probability = ProbabilityPossible
		case "Likely":
			probability = ProbabilityLikely
		case "Scheduled":
			probability = ProbabilityScheduled
		default:
			err = fmt.Errorf("unrecognized probability: %q", probability)
		}

		var startAt time.Time
		switch cells[3] {
		case "Undetermined":
			// zero value is correct
		default:
			err = fmt.Errorf("unable to parse time %q", cells[3])
		}

		out = append(out, ProgramSchedule{
			Class:             cells[0],
			ProgramType:       cells[1],
			Probability:       probability,
			ExpectedStartTime: startAt,
		})
	})

	if err != nil {
		return nil, err
	} else {
		return out, nil
	}
}
