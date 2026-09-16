package calendar

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/emersion/go-ical"
)

// httpClient bounds how long one feed may take. Google feeds carry the whole
// calendar history and can reach a few megabytes, which is still quick.
var httpClient = &http.Client{Timeout: 20 * time.Second}

// maxFeedBytes caps a feed body so a wrong URL cannot exhaust memory.
const maxFeedBytes = 32 << 20

// fetchFeed downloads and parses one ICS feed. Any non-2xx status is an error,
// unlike the shared httpclient package, because a feed host's error page would
// otherwise be parsed as an empty calendar.
func fetchFeed(url string) (*ical.Calendar, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/calendar, */*;q=0.5")
	req.Header.Set("User-Agent", "trmnl-server-go")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("feed returned HTTP %d", resp.StatusCode)
	}

	cal, err := ical.NewDecoder(io.LimitReader(resp.Body, maxFeedBytes)).Decode()
	if err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}
	return cal, nil
}
