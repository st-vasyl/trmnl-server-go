// Package calendar renders a one-day agenda merged from any number of
// iCalendar (ICS) feeds, such as an iCloud public calendar link or a Google
// Calendar secret address.
package calendar

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const pluginName = "calendar"

// Screen layouts.
const (
	layoutTimeline = "timeline" // hour grid with events as blocks (default)
	layoutList     = "list"     // one row per event
)

// Source is one ICS feed merged into the day view.
type Source struct {
	Name string // short label shown next to each event
	URL  string // http(s) or webcal address of the .ics feed
}

// Plugin renders one screen listing today's events from every configured feed.
type Plugin struct {
	loc     *time.Location
	layout  string
	sources []Source
	now     func() time.Time // replaced in tests
}

// New validates the configuration and returns the plugin. It fails on an empty
// calendar list, a calendar without a name, an address that is not http(s) or
// webcal, an unknown layout, or a time zone that cannot be loaded. An empty
// timezone means the server's local zone; an empty layout means "timeline".
func New(timezone, layout string, sources []Source) (*Plugin, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("calendar: no calendars configured")
	}
	loc := time.Local
	if timezone != "" {
		l, err := time.LoadLocation(timezone)
		if err != nil {
			return nil, fmt.Errorf("calendar: unknown timezone %q", timezone)
		}
		loc = l
	}
	layout = strings.ToLower(strings.TrimSpace(layout))
	switch layout {
	case "":
		layout = layoutTimeline
	case layoutTimeline, layoutList:
	default:
		return nil, fmt.Errorf("calendar: unknown layout %q (use %s or %s)", layout, layoutTimeline, layoutList)
	}
	p := &Plugin{loc: loc, layout: layout, now: time.Now}
	for i, s := range sources {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			return nil, fmt.Errorf("calendar: calendar %d has no name", i+1)
		}
		u, err := feedURL(s.URL)
		if err != nil {
			return nil, fmt.Errorf("calendar: calendar %q: %w", name, err)
		}
		p.sources = append(p.sources, Source{Name: name, URL: u})
	}
	return p, nil
}

// feedURL normalises a feed address: whitespace trimmed and webcal:// (what
// Apple Calendar hands out) rewritten to https://.
func feedURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	const webcal = "webcal://"
	if len(raw) >= len(webcal) && strings.EqualFold(raw[:len(webcal)], webcal) {
		raw = "https://" + raw[len(webcal):]
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("url %q: %w", raw, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("url %q must start with http://, https:// or webcal://", raw)
	}
	return raw, nil
}

func (p *Plugin) Name() string { return pluginName }

// Screens returns the single "calendar" screen.
func (p *Plugin) Screens() []string { return []string{pluginName} }

// Render fetches every feed and draws today's agenda to outputPath. A feed
// that fails is listed in the footer; nothing is written only when every feed
// fails.
func (p *Plugin) Render(screen, outputPath string, voltage float32) error {
	if screen != pluginName {
		return fmt.Errorf("calendar: unknown screen %q", screen)
	}
	now := p.now().In(p.loc)
	from, to := dayWindow(now, p.loc)
	events, failed, err := p.collect(from, to)
	if err != nil {
		return err
	}
	view := dayView{
		Day:      from,
		Now:      now,
		Events:   events,
		Failed:   failed,
		ShowTags: len(p.sources) > 1,
	}
	if p.layout == layoutList {
		return renderScreen(view, outputPath, voltage)
	}
	return renderTimeline(view, outputPath, voltage)
}

// collect fetches every feed and returns the merged events of [from, to) plus
// the names of the feeds that could not be fetched.
func (p *Plugin) collect(from, to time.Time) ([]Event, []string, error) {
	var events []Event
	var failed []string
	for _, s := range p.sources {
		cal, err := fetchFeed(s.URL)
		if err != nil {
			log.Error().Str("plugin", pluginName).Str("calendar", s.Name).Err(err).Msg("Failed to fetch calendar feed")
			failed = append(failed, s.Name)
			continue
		}
		events = append(events, eventsForDay(cal, s.Name, from, to, p.loc)...)
	}
	if len(failed) == len(p.sources) {
		return nil, failed, fmt.Errorf("calendar: all %d feeds failed", len(p.sources))
	}
	return merge(events), failed, nil
}
