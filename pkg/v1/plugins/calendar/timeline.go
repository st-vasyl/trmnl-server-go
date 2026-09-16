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
	tlBlockLineSize  = 16 // single-line blocks
	tlBlockPad       = 6
	tlBlockBar       = 4  // solid bar on the block's left edge
	tlBlockGap       = 3  // space between neighbouring blocks
	tlMinBlockH      = 19 // room for one line at tlBlockLineSize
	tlTwoLineH       = 40 // blocks at least this tall get time and title lines
	tlDither         = 3  // dot spacing of the block fill

	tlFooterSize     = 16
	tlFooterBaseline = 472

	// An empty day shows these hours. Otherwise the grid fits the day's
	// events, never narrower than tlMinWindowHours.
	defaultDayStartHour = 7
	defaultDayEndHour   = 19
	tlMinWindowHours    = 8
)

// block is a timed event placed in column col of cols within its overlap
// cluster.
type block struct {
	Event
	col, cols int
}

// placed is a block with its rectangle on the grid.
type placed struct {
	block
	rect image.Rectangle
}

// gridScale maps instants to grid rows.
type gridScale struct {
	winStart    time.Time
	pxPerHour   float64
	top, bottom int
}

func (g gridScale) y(t time.Time) int {
	return g.top + int(math.Round(t.Sub(g.winStart).Hours()*g.pxPerHour))
}

// timelineWindow returns the hour-aligned span of the grid for day. With no
// timed events it is the default hours. Otherwise it runs from the first
// event to the last, widened to at least tlMinWindowHours: first towards the
// default evening edge, then backwards, then forwards; always within the day.
func timelineWindow(events []Event, day time.Time) (start, end time.Time) {
	loc := day.Location()
	next := day.AddDate(0, 0, 1)
	hour := func(h int) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), h, 0, 0, 0, loc)
	}

	first, last := 24, 0
	for _, e := range events {
		if e.AllDay {
			continue
		}
		s := 0
		if e.Start.After(day) {
			s = e.Start.In(loc).Hour()
		}
		en := 24
		if e.End.Before(next) {
			t := e.End.In(loc)
			en = t.Hour()
			if hour(en).Before(t) {
				en++
			}
		}
		first, last = min(first, s), max(last, en)
	}
	if first >= last {
		return hour(defaultDayStartHour), hour(defaultDayEndHour)
	}

	if need := tlMinWindowHours - (last - first); need > 0 {
		grow := min(need, max(defaultDayEndHour-last, 0))
		last += grow
		need -= grow
		grow = min(need, first)
		first -= grow
		need -= grow
		last = min(last+need, 24)
	}
	return hour(first), hour(last)
}

// layoutColumns assigns overlapping timed events to side-by-side columns.
// Events are processed by start time; each one takes the first column that is
// free by then, and every event of an overlap cluster is told how many columns
// the cluster used so widths can be divided evenly. A zero-length event holds
// its column for a minute so it still gets a slot.
func layoutColumns(events []Event) []block {
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
		if !end.After(e.Start) {
			end = e.Start.Add(time.Minute)
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

// placeBlocks turns laid-out blocks (in start order) into rectangles on the
// grid between x0 and x0+width. Every block is at least minH tall; when that
// overflows into a later block sharing its horizontal span, the later block
// moves down by the difference, so consecutive short meetings stay stacked
// and readable at the cost of sitting a few pixels below their true time.
func placeBlocks(blocks []block, g gridScale, minH, x0, width int) []placed {
	out := make([]placed, 0, len(blocks))
	for _, b := range blocks {
		colW := width / b.cols
		x := image.Rect(x0+b.col*colW+tlBlockGap, 0, x0+(b.col+1)*colW-tlBlockGap, 1)

		y0 := max(g.y(b.Start), g.top)
		y1 := min(g.y(b.End), g.bottom)
		for _, p := range out {
			if p.rect.Min.X < x.Max.X && x.Min.X < p.rect.Max.X && p.rect.Max.Y > y0 {
				y0 = p.rect.Max.Y
			}
		}
		if y1 < y0+minH {
			y1 = y0 + minH
		}
		if y1 > g.bottom {
			y1 = g.bottom
			y0 = min(y0, y1-minH)
		}
		out = append(out, placed{block: b, rect: image.Rect(x.Min.X, y0, x.Max.X, y1)})
	}
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
	g := gridScale{
		winStart:  winStart,
		pxPerHour: float64(tlGridBottom-tlGridTop) / winEnd.Sub(winStart).Hours(),
		top:       tlGridTop,
		bottom:    tlGridBottom,
	}
	draw.Draw(img, image.Rect(gridX0-4, tlGridTop, gridX0-3, tlGridBottom+1), image.Black, image.Point{}, draw.Src)
	loc := v.Day.Location()
	for h := winStart.In(loc).Hour(); ; h++ {
		t := time.Date(v.Day.Year(), v.Day.Month(), v.Day.Day(), h, 0, 0, 0, loc)
		if t.After(winEnd) {
			break
		}
		y := g.y(t)
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

	// Event blocks.
	for _, p := range placeBlocks(layoutColumns(timed), g, tlMinBlockH, gridX0, gridX1-gridX0) {
		if err := drawBlock(img, p.Event, p.rect, from, to, v.ShowTags); err != nil {
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
// first line and the title on the second; a short one gets both on one line.
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
	maxW := r.Max.X - tlBlockPad - (r.Min.X + tlBlockBar + tlBlockPad)
	line, err := oneLineText(span, title, tlBlockLineSize, maxW)
	if err != nil {
		return err
	}
	return blockLine(img, r, line, tlBlockLineSize, r.Min.Y+tlBlockLineSize+1)
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
