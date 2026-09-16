package calendar

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"
	"strings"
	"time"
	"trmnl-server-go/pkg/v1/icons"
	"trmnl-server-go/pkg/v1/render"
)

// Timeline layout geometry. The day is drawn as an hour grid: labels down the
// left, an all-day strip above the grid, and one block per timed event placed
// proportionally to its span. Overlapping events share the width in columns.
const (
	tlHeaderSize     = 32
	tlHeaderBaseline = 40
	tlIconSize       = 36
	tlDividerY       = 52

	tlAllDayTop    = 58
	tlAllDayHeight = 30

	tlGridTop    = 98
	tlGridBottom = 440
	tlLabelColW  = 62 // hour labels left of the grid
	tlLabelSize  = 16

	tlBlockTitleSize = 18
	tlBlockTimeSize  = 14
	tlBlockLineSize  = 16 // largest single-line text; see blockMetrics
	tlBlockLineMin   = 11 // smallest single-line text that stays legible
	tlBlockPad       = 6
	tlBlockBar       = 4  // solid bar on the block's left edge
	tlBlockGap       = 3  // space between neighbouring blocks
	tlTwoLineH       = 40 // blocks at least this tall get time and title lines
	tlDither         = 3  // dot spacing of the block fill

	tlFooterSize     = 16
	tlFooterBaseline = 472

	// The grid always shows at least these hours; it stretches to include
	// earlier or later events.
	defaultDayStartHour = 7
	defaultDayEndHour   = 19
)

// block is a timed event placed in column col of cols within its overlap
// cluster.
type block struct {
	Event
	col, cols int
}

// blockMetrics derives the single-line text size and the minimum block height
// from the hour scale, so that a 30-minute event drawn at its natural height
// still holds a line of text. Twelve visible hours give 11px text; six give
// the full 16px.
func blockMetrics(pxPerHour float64) (lineSize float64, minH int) {
	slot := math.Round(pxPerHour / 2)
	lineSize = math.Min(math.Max(slot-3, tlBlockLineMin), tlBlockLineSize)
	return lineSize, int(lineSize) + 3
}

// lineSizeFor picks the single-line text size a block of the given height
// can hold, between the legible minimum and the full size.
func lineSizeFor(height int) float64 {
	return math.Min(math.Max(float64(height)-3, tlBlockLineMin), tlBlockLineSize)
}

// minBlockDuration is the span a block of minimum height covers on the grid;
// events hold their column for at least this long (see layoutColumns).
func minBlockDuration(pxPerHour float64) time.Duration {
	_, minH := blockMetrics(pxPerHour)
	return time.Duration(float64(minH) / pxPerHour * float64(time.Hour))
}

// timelineWindow returns the hour-aligned span of the grid for day: the
// default hours, widened to whole hours around any timed event and clamped to
// the day itself.
func timelineWindow(events []Event, day time.Time) (start, end time.Time) {
	loc := day.Location()
	next := day.AddDate(0, 0, 1)
	hour := func(h int) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), h, 0, 0, 0, loc)
	}
	start, end = hour(defaultDayStartHour), hour(defaultDayEndHour)
	for _, e := range events {
		if e.AllDay {
			continue
		}
		if e.Start.Before(start) {
			if e.Start.Before(day) {
				start = day
			} else {
				start = hour(e.Start.In(loc).Hour())
			}
		}
		if e.End.After(end) {
			if !e.End.Before(next) {
				end = next
			} else {
				h := e.End.In(loc)
				end = hour(h.Hour())
				if end.Before(h) {
					end = hour(h.Hour() + 1)
				}
			}
		}
	}
	return start, end
}

// layoutColumns assigns overlapping timed events to side-by-side columns.
// Events are processed by start time; each one takes the first column that is
// free by then, and every event of an overlap cluster is told how many columns
// the cluster used so widths can be divided evenly. Every event holds its
// column for at least minDur, the span its minimum block height covers on
// screen, so a short event is never drawn under the one that follows it.
func layoutColumns(events []Event, minDur time.Duration) []block {
	if minDur < time.Minute {
		minDur = time.Minute
	}
	sorted := append([]Event(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].Start.Equal(sorted[j].Start) {
			return sorted[i].Start.Before(sorted[j].Start)
		}
		return sorted[i].End.After(sorted[j].End)
	})

	var out []block
	clusterStart := 0
	var clusterEnd time.Time
	var colEnds []time.Time
	flush := func() {
		for i := clusterStart; i < len(out); i++ {
			out[i].cols = len(colEnds)
		}
		clusterStart = len(out)
		clusterEnd = time.Time{}
		colEnds = colEnds[:0]
	}
	for _, e := range sorted {
		end := e.End
		if floor := e.Start.Add(minDur); end.Before(floor) {
			end = floor
		}
		if len(colEnds) > 0 && !e.Start.Before(clusterEnd) {
			flush()
		}
		col := -1
		for i, free := range colEnds {
			if !free.After(e.Start) {
				col, colEnds[i] = i, end
				break
			}
		}
		if col < 0 {
			colEnds = append(colEnds, end)
			col = len(colEnds) - 1
		}
		if end.After(clusterEnd) {
			clusterEnd = end
		}
		out = append(out, block{Event: e, col: col})
	}
	flush()
	return out
}

// renderTimeline draws the day as an hour grid with event blocks.
func renderTimeline(v dayView, outputPath string, voltage float32) error {
	img := render.NewImage(screenW, screenH)
	black := color.Black
	from, to := v.Day, v.Day.AddDate(0, 0, 1)

	// Header. AddIcon takes the negated destination position (see render.AddIcon).
	if err := render.AddIcon(img, icons.Calendar, image.Point{-marginX, -8}, tlIconSize); err != nil {
		return err
	}
	if err := render.AddText(img, headerDate(v.Day), image.Point{marginX + 46, tlHeaderBaseline}, black, tlHeaderSize); err != nil {
		return err
	}
	draw.Draw(img, image.Rect(marginX, tlDividerY, screenW-marginX, tlDividerY+2), image.Black, image.Point{}, draw.Src)

	var allDay, timed []Event
	for _, e := range v.Events {
		if e.AllDay {
			allDay = append(allDay, e)
		} else {
			timed = append(timed, e)
		}
	}
	gridX0, gridX1 := marginX+tlLabelColW, screenW-marginX

	// All-day strip.
	if err := render.AddText(img, "all-day", image.Point{marginX, tlAllDayTop + 21}, black, tlLabelSize); err != nil {
		return err
	}
	if err := drawAllDayBlocks(img, allDay, gridX0, gridX1, tlAllDayTop, tlAllDayTop+tlAllDayHeight, v.ShowTags); err != nil {
		return err
	}

	// Hour grid: a rule between labels and grid, a dotted line per hour.
	winStart, winEnd := timelineWindow(timed, v.Day)
	pxPerHour := float64(tlGridBottom-tlGridTop) / winEnd.Sub(winStart).Hours()
	yOf := func(t time.Time) int {
		return tlGridTop + int(math.Round(t.Sub(winStart).Hours()*pxPerHour))
	}
	draw.Draw(img, image.Rect(gridX0-4, tlGridTop, gridX0-3, tlGridBottom+1), image.Black, image.Point{}, draw.Src)
	loc := v.Day.Location()
	for h := winStart.In(loc).Hour(); ; h++ {
		t := time.Date(v.Day.Year(), v.Day.Month(), v.Day.Day(), h, 0, 0, 0, loc)
		if t.After(winEnd) {
			break
		}
		y := yOf(t)
		render.AddDottedLine(img, gridX0, gridX1, y, 3)
		label := fmt.Sprintf("%d:00", h)
		w, err := render.TextWidth(label, tlLabelSize)
		if err != nil {
			return err
		}
		if err := render.AddText(img, label, image.Point{gridX0 - 10 - w, y + 6}, black, tlLabelSize); err != nil {
			return err
		}
	}

	// Event blocks. Text size and minimum height follow the hour scale, and
	// the minimum height, expressed as time, decides when neighbouring events
	// must share the width instead of stacking.
	gridW := gridX1 - gridX0
	_, minH := blockMetrics(pxPerHour)
	for _, b := range layoutColumns(timed, minBlockDuration(pxPerHour)) {
		s, e := b.Start, b.End
		if s.Before(winStart) {
			s = winStart
		}
		if e.After(winEnd) {
			e = winEnd
		}
		y0, y1 := yOf(s), yOf(e)
		if y1 < y0+minH {
			y1 = y0 + minH
		}
		if y1 > tlGridBottom {
			y1 = tlGridBottom
			if y0 > y1-minH {
				y0 = y1 - minH
			}
		}
		colW := gridW / b.cols
		r := image.Rect(gridX0+b.col*colW+tlBlockGap, y0, gridX0+(b.col+1)*colW-tlBlockGap, y1)
		if err := drawBlock(img, b.Event, r, from, to, v.ShowTags); err != nil {
			return err
		}
	}

	if len(v.Events) == 0 {
		if err := drawCentered(img, "No events today", (tlGridTop+tlGridBottom)/2+10, 26); err != nil {
			return err
		}
	}

	// Footer: feed problems on the left, render time on the right.
	if len(v.Failed) > 0 {
		if err := render.AddIcon(img, icons.Warning, image.Point{-marginX, -(tlFooterBaseline - 20)}, 22); err != nil {
			return err
		}
		msg := strings.Join(v.Failed, ", ") + " unavailable"
		if err := render.AddText(img, msg, image.Point{marginX + 30, tlFooterBaseline}, black, tlFooterSize); err != nil {
			return err
		}
	}
	updated := "Updated " + v.Now.Format("15:04")
	w, err := render.TextWidth(updated, tlFooterSize)
	if err != nil {
		return err
	}
	if err := render.AddText(img, updated, image.Point{screenW - marginX - w, tlFooterBaseline}, black, tlFooterSize); err != nil {
		return err
	}

	return render.WriteFile(outputPath, img, voltage)
}

// drawAllDayBlocks lays all-day events side by side across the strip. Up to
// three are shown; beyond that the last slot counts the rest.
func drawAllDayBlocks(img *image.RGBA, events []Event, x0, x1, y0, y1 int, showTag bool) error {
	if len(events) == 0 {
		return nil
	}
	labels := make([]string, 0, 3)
	for _, e := range events {
		labels = append(labels, blockTitle(e, showTag))
	}
	if len(labels) > 3 {
		labels = append(labels[:2], fmt.Sprintf("+%d more", len(labels)-2))
	}
	w := (x1 - x0) / len(labels)
	for i, label := range labels {
		r := image.Rect(x0+i*w+tlBlockGap, y0+2, x0+(i+1)*w-tlBlockGap, y1-2)
		fillBlock(img, r)
		if err := blockLine(img, r, label, tlBlockLineSize, r.Min.Y+19); err != nil {
			return err
		}
	}
	return nil
}

// drawBlock draws one timed event: a tall block gets the time span on the
// first line and the title on the second; a short one gets both on one line
// sized to the block's height.
func drawBlock(img *image.RGBA, e Event, r image.Rectangle, from, to time.Time, showTag bool) error {
	fillBlock(img, r)
	span := timeLabel(e, from, to)
	title := blockTitle(e, showTag)
	if r.Dy() >= tlTwoLineH {
		if err := blockLine(img, r, span, tlBlockTimeSize, r.Min.Y+tlBlockTimeSize+3); err != nil {
			return err
		}
		return blockLine(img, r, title, tlBlockTitleSize, r.Min.Y+tlBlockTimeSize+tlBlockTitleSize+8)
	}
	lineSize := lineSizeFor(r.Dy())
	maxW := r.Max.X - tlBlockPad - (r.Min.X + tlBlockBar + tlBlockPad)
	line, err := oneLineText(span, title, lineSize, maxW)
	if err != nil {
		return err
	}
	return blockLine(img, r, line, lineSize, r.Min.Y+int(lineSize)+1)
}

// oneLineText is the text of a single-line block: the time span and title
// when both fit in maxW, otherwise the title alone (shortened if needed),
// since the block's position already shows the time.
func oneLineText(span, title string, size float64, maxW int) (string, error) {
	line := span + " · " + title
	w, err := render.TextWidth(line, size)
	if err != nil {
		return "", err
	}
	if w <= maxW {
		return line, nil
	}
	return truncate(title, size, maxW)
}

// fillBlock draws the dotted body of a block and its solid left bar.
func fillBlock(img *image.RGBA, r image.Rectangle) {
	render.AddDitherRect(img, r, tlDither)
	draw.Draw(img, image.Rect(r.Min.X, r.Min.Y, r.Min.X+tlBlockBar, r.Max.Y), image.Black, image.Point{}, draw.Src)
}

// blockLine writes one line of text inside r at baseline y, shortened to fit,
// on a white strip so the dots do not run through the letters.
func blockLine(img *image.RGBA, r image.Rectangle, text string, size float64, y int) error {
	x := r.Min.X + tlBlockBar + tlBlockPad
	maxW := r.Max.X - tlBlockPad - x
	if maxW <= 0 {
		return nil
	}
	text, err := truncate(text, size, maxW)
	if err != nil || text == "" {
		return err
	}
	w, err := render.TextWidth(text, size)
	if err != nil {
		return err
	}
	strip := image.Rect(x-2, y-int(size)+2, x+w+2, y+4).Intersect(r)
	draw.Draw(img, strip, image.White, image.Point{}, draw.Src)
	return render.AddText(img, text, image.Point{x, y}, color.Black, size)
}

// blockTitle is the event title, with its calendar tag when tags are shown.
func blockTitle(e Event, showTag bool) string {
	if showTag {
		return e.Title + " [" + e.Calendar + "]"
	}
	return e.Title
}
