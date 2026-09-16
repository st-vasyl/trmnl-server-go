package calendar

import (
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"trmnl-server-go/pkg/v1/render"

	"github.com/emersion/go-ical"
	"github.com/rs/zerolog/log"
)

// Event is one entry of the day view, normalised from a VEVENT (or one
// occurrence of a recurring VEVENT). Times are in the configured zone.
type Event struct {
	Calendar string // display name of the feed it came from
	UID      string
	Title    string
	Start    time.Time
	End      time.Time // exclusive; equals Start for zero-length events
	AllDay   bool      // a DATE event, or a timed event covering the whole day
}

// untitled is shown for a VEVENT without a SUMMARY.
const untitled = "(No title)"

// dateProps are the VEVENT properties that carry a TZID parameter.
var dateProps = []string{
	ical.PropDateTimeStart, ical.PropDateTimeEnd, ical.PropRecurrenceID,
	ical.PropExceptionDates, ical.PropRecurrenceDates,
}

// warnedTZIDs remembers unloadable zone names so a feed with hundreds of
// events logs each name once, not once per event per tick.
var warnedTZIDs sync.Map

// dayWindow returns the half-open [midnight, next midnight) window that
// contains t in loc.
func dayWindow(t time.Time, loc *time.Location) (from, to time.Time) {
	t = t.In(loc)
	from = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	return from, from.AddDate(0, 0, 1)
}

// overlaps reports whether an event touches [from, to). Zero-length events
// count when their instant lies inside the window.
func overlaps(start, end, from, to time.Time) bool {
	if !start.Before(to) {
		return false
	}
	if end.After(from) {
		return true
	}
	return end.Equal(start) && !start.Before(from)
}

// eventsForDay expands cal into the events that overlap [from, to), including
// occurrences of recurring events, with RECURRENCE-ID overrides applied and
// cancelled or declined entries dropped. Malformed events are logged and
// skipped.
func eventsForDay(cal *ical.Calendar, calName string, from, to time.Time, loc *time.Location) []Event {
	events := cal.Events()
	for _, ev := range events {
		stripUnknownTZIDs(ev.Component)
	}
	owner := feedOwner(cal)

	// An override (a VEVENT with RECURRENCE-ID) replaces the generated
	// occurrence of its series that starts at that instant, even when the
	// override itself is cancelled or moved to another day.
	overridden := map[string]bool{}
	for _, ev := range events {
		if prop := ev.Props.Get(ical.PropRecurrenceID); prop != nil {
			if t, err := prop.DateTime(loc); err == nil {
				overridden[instanceKey(uid(ev), t)] = true
			}
		}
	}

	var out []Event
	for _, ev := range events {
		if status, err := ev.Status(); err == nil && status == ical.EventCancelled {
			continue
		}
		if declinedBy(ev, owner) {
			continue
		}
		start, err := ev.DateTimeStart(loc)
		if err != nil || start.IsZero() {
			log.Warn().Str("plugin", pluginName).Str("calendar", calName).Str("uid", uid(ev)).Err(err).Msg("Skipping event without a valid start")
			continue
		}
		end, err := ev.DateTimeEnd(loc)
		if err != nil {
			log.Warn().Str("plugin", pluginName).Str("calendar", calName).Str("uid", uid(ev)).Err(err).Msg("Skipping event with an invalid end")
			continue
		}
		if end.Before(start) {
			end = start
		}

		base := Event{
			Calendar: calName,
			UID:      uid(ev),
			Title:    title(ev),
			AllDay:   ev.Props.Get(ical.PropDateTimeStart).ValueType() == ical.ValueDate,
		}

		isSeries := ev.Props.Get(ical.PropRecurrenceRule) != nil && ev.Props.Get(ical.PropRecurrenceID) == nil
		if !isSeries {
			if overlaps(start, end, from, to) {
				out = append(out, finish(base, start, end, from, to, loc))
			}
			continue
		}

		set, err := ev.RecurrenceSet(loc)
		if err != nil {
			log.Warn().Str("plugin", pluginName).Str("calendar", calName).Str("uid", base.UID).Err(err).Msg("Skipping event with an invalid recurrence rule")
			continue
		}
		dur := end.Sub(start)
		days := int(math.Round(dur.Hours() / 24))
		// Occurrences that begin before the day can still reach into it.
		for _, s := range set.Between(from.Add(-dur), to, true) {
			if overridden[instanceKey(base.UID, s)] {
				continue
			}
			e := s.Add(dur)
			if base.AllDay {
				// Whole days, not 24-hour spans: DST days are 23 or 25 hours.
				e = s.AddDate(0, 0, days)
			}
			if overlaps(s, e, from, to) {
				out = append(out, finish(base, s, e, from, to, loc))
			}
		}
	}
	return out
}

// finish fills in the times of an event in the configured zone and promotes a
// timed event that covers the whole day to the all-day group.
func finish(e Event, start, end, from, to time.Time, loc *time.Location) Event {
	e.Start = start.In(loc)
	e.End = end.In(loc)
	if !start.After(from) && !end.Before(to) {
		e.AllDay = true
	}
	return e
}

// instanceKey identifies one occurrence of a series by UID and instant.
func instanceKey(uid string, t time.Time) string {
	return uid + "|" + t.UTC().Format(time.RFC3339)
}

func uid(ev ical.Event) string {
	s, _ := ev.Props.Text(ical.PropUID)
	return s
}

// title is the SUMMARY with symbols the font cannot draw removed (emoji are
// common in shared calendars) and whitespace collapsed; empty becomes untitled.
func title(ev ical.Event) string {
	s, err := ev.Props.Text(ical.PropSummary)
	if err != nil {
		return untitled
	}
	s = strings.Join(strings.Fields(render.Printable(s)), " ")
	if s == "" {
		return untitled
	}
	return s
}

// feedOwner returns the lower-cased address the feed belongs to, taken from
// Google's X-WR-CALNAME header when it is an address, or "" when unknown.
func feedOwner(cal *ical.Calendar) string {
	name, err := cal.Props.Text("X-WR-CALNAME")
	if err != nil || !strings.Contains(name, "@") {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(name))
}

// declinedBy reports whether owner is listed as an attendee who declined the
// event. Google keeps such invitations in the feed but hides them in its UI.
func declinedBy(ev ical.Event, owner string) bool {
	if owner == "" {
		return false
	}
	for _, att := range ev.Props.Values(ical.PropAttendee) {
		if !strings.EqualFold(att.Params.Get(ical.ParamParticipationStatus), "DECLINED") {
			continue
		}
		addr := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(att.Value)), "mailto:")
		if addr == owner {
			return true
		}
	}
	return false
}

// stripUnknownTZIDs removes TZID parameters that time.LoadLocation cannot
// resolve (Outlook's Windows zone names, for instance) so the times parse as
// floating in the configured zone instead of failing the whole event.
func stripUnknownTZIDs(comp *ical.Component) {
	for _, name := range dateProps {
		props := comp.Props.Values(name)
		for i := range props {
			tzid := props[i].Params.Get(ical.PropTimezoneID)
			if tzid == "" {
				continue
			}
			if _, err := time.LoadLocation(tzid); err != nil {
				if _, seen := warnedTZIDs.LoadOrStore(tzid, true); !seen {
					log.Warn().Str("plugin", pluginName).Str("tzid", tzid).Msg("Unknown time zone in feed; using the configured zone")
				}
				props[i].Params.Del(ical.PropTimezoneID)
			}
		}
	}
}

// merge orders events for display (all-day first, then by start, then title)
// and drops duplicates, keeping the first calendar an event was seen in. The
// same UID at the same start is one event even across feeds; occurrences of a
// series keep distinct starts and so stay distinct.
func merge(events []Event) []Event {
	seen := map[string]bool{}
	out := make([]Event, 0, len(events))
	for _, e := range events {
		key := dedupeKey(e)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.AllDay != b.AllDay {
			return a.AllDay
		}
		if !a.Start.Equal(b.Start) {
			return a.Start.Before(b.Start)
		}
		return a.Title < b.Title
	})
	return out
}

func dedupeKey(e Event) string {
	if e.UID != "" {
		return instanceKey(e.UID, e.Start)
	}
	return "title|" + strings.ToLower(e.Title) + "|" + e.Start.UTC().Format(time.RFC3339) + "|" + e.End.UTC().Format(time.RFC3339)
}
