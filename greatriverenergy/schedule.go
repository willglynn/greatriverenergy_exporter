package greatriverenergy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Schedule struct {
	ConservationGauge ConservationStatus `json:"conservationGauge"`
	Today             []ProgramSchedule  `json:"today"`
	NextDay           []ProgramSchedule  `json:"nextDay"`
	LastUpdated       time.Time          `json:"lastUpdated"`
}

type ProgramSchedule struct {
	Class             string      `json:"class"`
	ProgramType       string      `json:"programType"`
	Probability       Probability `json:"probability"`
	ExpectedStartTime time.Time   `json:"expectedStartTime,omitempty"`
	ExpectedEndTime   time.Time   `json:"expectedEndTime,omitempty"`
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

	var today time.Time
	dateTime := doc.Find("#ContentPlaceHolder2_DateTime_Label").Text()
	if dateTime, found := strings.CutSuffix(dateTime, " CPT"); !found {
		return nil, fmt.Errorf("current date/time did not end in \"CPT\": %q", dateTime)
	} else if today, err = time.ParseInLocation("Mon Jan _2, 2006 - 03:04 PM", dateTime, tz); err != nil {
		return nil, fmt.Errorf("failed to parse date/time %q: %v", dateTime, err)
	}
	nextDay := today.AddDate(0, 0, 1)

	todaySchedule, err := parseScheduleTable(doc.Find("#ContentPlaceHolder2_TodaySched_Table"), today)
	if err != nil {
		return nil, fmt.Errorf("scrape failure: failed to parse Today table: %v", err)
	}

	nextDaySchedule, err := parseScheduleTable(doc.Find("#ContentPlaceHolder2_NextDaySched_Table"), nextDay)
	if err != nil {
		return nil, fmt.Errorf("scrape failure: failed to parse Next Day table: %v", err)
	}

	var lastUpdated time.Time
	if lastUpdatedStr, found := strings.CutSuffix(doc.Find("#ContentPlaceHolder2_LastUdpated_Label").Text(), " CPT"); !found {
		return nil, fmt.Errorf("last updated date/time did not end in \"CPT\": %q", lastUpdatedStr)
	} else if lastUpdated, err = time.ParseInLocation("01/02/2006 03:04 PM", lastUpdatedStr, tz); err != nil {
		return nil, fmt.Errorf("failed to parse last updated time: %v", err)
	}

	return &Schedule{
		ConservationGauge: conservationGauge,
		Today:             todaySchedule,
		NextDay:           nextDaySchedule,
		LastUpdated:       lastUpdated,
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

		var startAt, endAt time.Time
		if cells[3] == "Undetermined" {
			// zero value is correct
		} else if parts := strings.Split(cells[3], " - "); len(parts) == 2 {
			ymd := day.Format("2006-01-02 ")
			start, err := time.ParseInLocation("2006-01-02 03:04 PM", ymd+parts[0], day.Location())
			if err == nil {
				startAt = start
			} else {
				log.Printf("warning: failed to parse start time %q", parts[0])
			}

			end, err := time.ParseInLocation("2006-01-02 03:04 PM", ymd+parts[1], day.Location())
			if err == nil {
				endAt = end
			} else {
				log.Printf("warning: failed to parse end time %q", parts[1])
			}
		} else {
			log.Printf("warning: failed to parse time %q", cells[3])
		}

		out = append(out, ProgramSchedule{
			Class:             cells[0],
			ProgramType:       cells[1],
			Probability:       probability,
			ExpectedStartTime: startAt,
			ExpectedEndTime:   endAt,
		})
	})

	if err != nil {
		return nil, err
	} else {
		return out, nil
	}
}
