package currency

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"trmnl-server-go/pkg/v1/icons"
	"trmnl-server-go/pkg/v1/render"
)

const (
	screenW, screenH = 800, 480
	cellW, cellH     = screenW / 2, screenH / 2

	// flatThreshold is the percent change below which a move prints as 0.00%
	// and is shown with the flat trend icon.
	flatThreshold = 0.005
)

// renderScreen draws up to four pairs in a two-by-two grid. Each cell shows
// the pair label and a sparkline on top, the rate in large type in the
// middle, and the day-over-day change with a trend icon at the bottom.
func renderScreen(pairs []Pair, stats []pairStats, outputPath string, voltage float32) error {
	img := render.NewImage(screenW, screenH)

	// Thin dividers between the cells.
	draw.Draw(img, image.Rect(cellW-1, 0, cellW+1, screenH), image.Black, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, cellH-1, screenW, cellH+1), image.Black, image.Point{}, draw.Src)

	for i, pair := range pairs {
		cx := (i % 2) * cellW
		cy := (i / 2) * cellH
		if err := drawCell(img, cx, cy, pair, stats[i]); err != nil {
			return err
		}
	}
	return render.WriteFile(outputPath, img, voltage)
}

// drawCell draws one pair into the cell whose top-left corner is (cx, cy).
func drawCell(img *image.RGBA, cx, cy int, pair Pair, st pairStats) error {
	if err := render.AddText(img, pair.String(), image.Point{cx + 20, cy + 55}, color.Black, 32); err != nil {
		return err
	}
	if len(st.History) >= 2 {
		if err := render.AddSparkline(img, st.History, 140, 70, image.Point{cx + 200, cy + 15}); err != nil {
			return err
		}
	}
	if err := render.AddText(img, formatRate(st.Rate), image.Point{cx + 20, cy + 150}, color.Black, 60); err != nil {
		return err
	}
	// AddIcon takes the negated destination position (see render.AddIcon).
	if err := render.AddIcon(img, trendIcon(st.Change), image.Point{-(cx + 20), -(cy + 175)}, 44); err != nil {
		return err
	}
	return render.AddText(img, formatChange(st.Change), image.Point{cx + 75, cy + 212}, color.Black, 30)
}

// formatRate prints FX rates with four decimals, or two once the rate is at
// least 100 (JPY-style quotes) so the text stays short.
func formatRate(v float64) string {
	if v >= 100 {
		return fmt.Sprintf("%.2f", v)
	}
	return fmt.Sprintf("%.4f", v)
}

// formatChange prints a signed percent; moves too small to show are "0.00%".
func formatChange(pct float64) string {
	switch {
	case pct >= flatThreshold:
		return fmt.Sprintf("+%.2f%%", pct)
	case pct <= -flatThreshold:
		return fmt.Sprintf("%.2f%%", pct)
	default:
		return "0.00%"
	}
}

// trendIcon picks the up, down, or flat glyph for a percent change.
func trendIcon(pct float64) string {
	switch {
	case pct >= flatThreshold:
		return icons.TrendUp
	case pct <= -flatThreshold:
		return icons.TrendDown
	default:
		return icons.TrendFlat
	}
}
