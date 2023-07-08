package greatriverenergy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type History struct {
	StartOn time.Time      `json:"startOn"`
	EndOn   time.Time      `json:"endOn"`
	Events  []HistoryEvent `json:"events"`
}

type HistoryEvent struct {
	ProgramName string `json:"programName"`
	Hours       float64
	StartAt     time.Time `json:"startAt"`
	EndAt       time.Time `json:"endAt"`
}

type HistoryType string

const (
	// "Residential"
	HistoryTypeR HistoryType = "RES"
	// "Commercial and Industrial"
	HistoryTypeCI HistoryType = "CI"
	// "Critical Peak Pricing"
	HistoryTypeCriticalPeakPricing HistoryType = "CPP"
	// "Public Appeal for Conservation"
	//
	// (This seems to not work.)
	HistoryTypePublicAppeal HistoryType = "PA"
)

func (c Client) historyFormValues(ctx context.Context, historyType HistoryType, startOn, endOn time.Time) (url.Values, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://lmguide.grenergy.com/HistoryForm.aspx", nil)

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

	form := doc.Find("form#form1")
	if form.AttrOr("action", "") != "./HistoryForm.aspx" {
		return url.Values{}, fmt.Errorf("unable to find form")
	}

	values := make(url.Values)
	form.Find("input, select").Each(func(_ int, selection *goquery.Selection) {
		name := selection.AttrOr("name", "")
		value := selection.AttrOr("value", "")

		if strings.Contains(name, "Reset_Button") {
			// don't click
			return
		} else if strings.Contains(name, "Guide") {
			value = string(historyType)
		} else if strings.Contains(name, "StartDate") {
			value = startOn.Format("01/02/2006")
		} else if strings.Contains(name, "EndDate") {
			value = endOn.Format("01/02/2006")
		}

		values.Add(name, value)
	})

	return values, nil
}

func toMidnight(t time.Time) time.Time {
	y, m, d := t.In(tz).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, tz)
}

func (c Client) History(ctx context.Context, historyType HistoryType, startOn, endOn time.Time) (*History, error) {
	// Clamp midnight to midnight local time
	startOn = toMidnight(startOn)
	endOn = toMidnight(endOn)

	// Get the form values we need to submit
	params, err := c.historyFormValues(ctx, historyType, startOn, endOn)
	if err != nil {
		return nil, fmt.Errorf("error loading history form: %v", err)
	}

	// Submit them
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://lmguide.grenergy.com/HistoryForm.aspx", strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected status code 200, got %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	_ = os.WriteFile("dump.html", body, 0644)

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	table := doc.Find("table#ContentPlaceHolder2_HistoryResults_Table")
	if len(table.Nodes) != 1 {
		return nil, fmt.Errorf("unable to find history table")
	}

	var events []HistoryEvent
	table.Find("tr.BodyText_noSpaces").Each(func(_ int, selection *goquery.Selection) {
		if err != nil {
			return
		}

		cells := selection.Find("td").Map(func(_ int, td *goquery.Selection) string {
			return td.Text()
		})
		if len(cells) != 5 {
			err = fmt.Errorf("expected 5 cells in each history row, got %v", len(cells))
			return
		}

		_, dateErr := time.ParseInLocation("01/02/2006", cells[0], tz)
		if dateErr != nil {
			err = fmt.Errorf("error parsing row ymd: %v", dateErr)
			return
		}

		startAt, dateErr := time.ParseInLocation("01/02/2006 15:04", cells[0]+" "+cells[2], tz)
		if dateErr != nil {
			err = fmt.Errorf("error parsing start time: %v", dateErr)
			return
		}

		hours, hoursErr := strconv.ParseFloat(cells[4], 64)
		if hoursErr != nil {
			err = fmt.Errorf("error parsing hours: %v", hoursErr)
		}
		seconds := math.Round(hours * 3600)
		endAt := startAt.Add(time.Duration(seconds) * time.Second)

		events = append(events, HistoryEvent{
			ProgramName: cells[1],
			Hours:       hours,
			StartAt:     startAt,
			EndAt:       endAt,
		})
	})

	// It's possible to ask for dates which might be in the future, and it's possible the
	// API would return information for the future (i.e. today which hasn't ended yet)
	// Make sure the endOn we return indicates which days are actually complete
	if thisMorningAtMidnight := toMidnight(time.Now()); thisMorningAtMidnight.Before(endOn) {
		endOn = thisMorningAtMidnight
	}

	return &History{
		StartOn: startOn,
		EndOn:   endOn,
		Events:  events,
	}, nil
}
