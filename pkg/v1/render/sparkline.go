package render

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	vgdraw "gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

// AddSparkline draws values as a bare line (no axes, ticks or grid) exactly
// width×height pixels in size with its top-left corner at point. Unlike
// AddChart and AddIcon, point is the destination position on img.
func AddSparkline(img *image.RGBA, values []float64, width, height int, point image.Point) error {
	if len(values) < 2 {
		return fmt.Errorf("sparkline needs at least 2 values, got %d", len(values))
	}

	pts := make(plotter.XYs, len(values))
	for i, v := range values {
		pts[i].X = float64(i)
		pts[i].Y = v
	}
	line, err := plotter.NewLine(pts)
	if err != nil {
		return err
	}
	line.Color = color.Black
	line.Width = vg.Points(2)

	p := plot.New()
	p.HideAxes()
	p.Add(line)

	// At 72 DPI one vg point is one pixel, so the canvas is exactly width×height.
	c := vgimg.NewWith(
		vgimg.UseWH(vg.Points(float64(width)), vg.Points(float64(height))),
		vgimg.UseDPI(72),
	)
	p.Draw(vgdraw.New(c))

	dst := image.Rect(point.X, point.Y, point.X+width, point.Y+height)
	draw.Draw(img, dst, c.Image(), image.Point{}, draw.Over)
	return nil
}
