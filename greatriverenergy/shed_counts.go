package greatriverenergy

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type ShedCounts struct {
	Table map[string]int

	// The ymd on which the counts were reset
	LastResetOn time.Time
}

func (c *Client) ShedCounts(ctx context.Context) (*ShedCounts, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://lmguide.grenergy.com/ShedCount.aspx", nil)
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

	table := make(map[string]int)

	doc.Find("#ContentPlaceHolder2_ShedCounts_Table tr.BodyText_noSpaces").Each(func(_ int, selection *goquery.Selection) {
		if err != nil {
			return
		}

		cells := selection.Find("td")
		name := cells.Eq(0).Text()
		count := cells.Eq(2).Text()
		if name == "" || count == "" {
			err = fmt.Errorf("scrape failure: table cell was empty")
			return
		}

		parsedCount, parseErr := strconv.Atoi(count)
		if parseErr != nil {
			err = fmt.Errorf("scrape failure: unable to parse count for %q: %v", name, parseErr)
			return
		}

		table[name] = parsedCount
	})
	if err != nil {
		return nil, err
	}

	resetCount := doc.Find("#ContentPlaceHolder2_ShedCountReset_Label").Text()
	if resetCount == "" {
		return nil, fmt.Errorf("scrape failure: unable to find reset count")
	}
	parsedResetCount, err := time.ParseInLocation("01/02/2006", resetCount, tz)
	if err != nil {
		return nil, fmt.Errorf("scrape failure: unable to parse reset count: %v", err)
	}

	return &ShedCounts{
		Table:       table,
		LastResetOn: parsedResetCount,
	}, nil
}
