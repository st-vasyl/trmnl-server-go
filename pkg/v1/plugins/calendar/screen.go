package calendar

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"time"
	"trmnl-server-go/pkg/v1/icons"
	"trmnl-server-go/pkg/v1/render"
)

const (
	screenW, screenH = 800, 480
	marginX          = 20

	headerSize     = 40
	headerBaseline = 52
	dividerY       = 72

	// Rows run from the divider down to the footer; eight fit comfortably.
	firstRowBaseline = 116
	rowHeight        = 42
	rowSize          = 26
	tagSize          = 20
	maxRows          = 8

	// timeColumnWidth fits "00:00-00:00" at rowSize in a monospace face.
	timeColumnWidth = 190

	footerSize     = 20
	footerBaseline = 462
)

// dayView is everything the screen draws.
type dayView struct {
	Day      time.Time // start of the day in the configured zone
	Now      time.Time // render time, shown in the footer
	Events   []Event   // already merged and ordered
	Failed   []string  // calendars whose feed could not be fetched
	ShowTags bool      // label each row with its calendar (more than one feed)
}

// renderScreen draws the agenda: a date header, one row per event with the
// time span, title and calendar tag, an overflow line when the day does not
// fit, and a footer with feed problems and the render time.
func renderScreen(v dayView, outputPath string, voltage float32) error {
	img := render.NewImage(screenW, screenH)
	black := color.Black

	// Header. AddIcon takes the negated destination position (see render.AddIcon).
	if err := render.AddIcon(img, icons.Calendar, image.Point{-marginX, -12}, 44); err != nil {
		return err
	}
	if err := render.AddText(img, headerDate(v.Day), image.Point{marginX + 56, headerBaseline}, black, headerSize); err != nil {
		return err
	}
	draw.Draw(img, image.Rect(marginX, dividerY, screenW-marginX, dividerY+2), image.Black, image.Point{}, draw.Src)

	if len(v.Events) == 0 {
		if err := drawCentered(img, "No events today", 270, 30); err != nil {
			return err
		}
	} else {
		rows, more := v.Events, 0
		if len(rows) > maxRows {
			rows, more = rows[:maxRows-1], len(rows)-(maxRows-1)
		}
		from, to := v.Day, v.Day.AddDate(0, 0, 1)
		for i, e := range rows {
			if err := drawRow(img, e, from, to, firstRowBaseline+i*rowHeight, v.ShowTags); err != nil {
				return err
			}
		}
		if more > 0 {
			y := firstRowBaseline + len(rows)*rowHeight
			if err := render.AddText(img, fmt.Sprintf("+%d more", more), image.Point{marginX + timeColumnWidth, y}, black, rowSize); err != nil {
				return err
			}
		}
	}

	// Footer: feed problems on the left, render time on the right.
	if len(v.Failed) > 0 {
		if err := render.AddIcon(img, icons.Warning, image.Point{-marginX, -(footerBaseline - 26)}, 28); err != nil {
			return err
		}
		msg := strings.Join(v.Failed, ", ") + " unavailable"
		if err := render.AddText(img, msg, image.Point{marginX + 36, footerBaseline}, black, footerSize); err != nil {
			return err
		}
	}
	updated := "Updated " + v.Now.Format("15:04")
	w, err := render.TextWidth(updated, footerSize)
	if err != nil {
		return err
	}
	if err := render.AddText(img, updated, image.Point{screenW - marginX - w, footerBaseline}, black, footerSize); err != nil {
		return err
	}

	return render.WriteFile(outputPath, img, voltage)
}

// drawRow draws one event at baseline y: time span, title (shortened to the
// space left before the tag) and, when tags are on, the calendar name.
func drawRow(img *image.RGBA, e Event, from, to time.Time, y int, showTag bool) error {
	black := color.Black
	if err := render.AddText(img, timeLabel(e, from, to), image.Point{marginX, y}, black, rowSize); err != nil {
		return err
	}
	titleX := marginX + timeColumnWidth
	titleEnd := screenW - marginX
	if showTag {
		tag := "[" + e.Calendar + "]"
		tagW, err := render.TextWidth(tag, tagSize)
		if err != nil {
			return err
		}
		tagX := screenW - marginX - tagW
		if err := render.AddText(img, tag, image.Point{tagX, y}, black, tagSize); err != nil {
			return err
		}
		titleEnd = tagX - 12
	}
	title, err := truncate(e.Title, rowSize, titleEnd-titleX)
	if err != nil {
		return err
	}
	return render.AddText(img, title, image.Point{titleX, y}, black, rowSize)
}

func drawCentered(img *image.RGBA, text string, y int, size float64) error {
	w, err := render.TextWidth(text, size)
	if err != nil {
		return err
	}
	return render.AddText(img, text, image.Point{(screenW - w) / 2, y}, color.Black, size)
}

// headerDate formats the day as "Wednesday, 16 September".
func headerDate(day time.Time) string {
	return day.Format("Monday, 2 January")
}

// timeLabel formats an event's span within the day: "All day", "09:00-10:30",
// a bare "12:00" for a zero-length event, and 00:00 or 24:00 in place of a
// start or end that falls outside the day.
func timeLabel(e Event, from, to time.Time) string {
	if e.AllDay {
		return "All day"
	}
	start := "00:00"
	if !e.Start.Before(from) {
		start = e.Start.Format("15:04")
	}
	if e.End.Equal(e.Start) {
		return start
	}
	end := "24:00"
	if e.End.Before(to) {
		end = e.End.Format("15:04")
	}
	return start + "-" + end
}

// truncate shortens s with a "..." suffix until it fits in maxW pixels at the
// given font size. It returns "" when not even the suffix fits.
func truncate(s string, size float64, maxW int) (string, error) {
	w, err := render.TextWidth(s, size)
	if err != nil {
		return "", err
	}
	if w <= maxW {
		return s, nil
	}
	runes := []rune(s)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := strings.TrimRight(string(runes), " ") + "..."
		w, err := render.TextWidth(candidate, size)
		if err != nil {
			return "", err
		}
		if w <= maxW {
			return candidate, nil
		}
	}
	return "", nil
}
