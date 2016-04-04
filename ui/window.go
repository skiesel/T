// Copyright © 2016, The T Authors.

package ui

import (
	"image"
	"image/color"
	"image/draw"
	"time"

	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
)

// A handler is an interactive portion of a window
// that can receive keyboard and mouse events.
//
// Handlers gain focus when the mouse hovers over them
// and they maintain focus until the mouse moves off of them.
// However, during a mouse drag event,
// when the pointer moves while a button is held,
// the handler maintains focus
// even if the pointer moves off of the handler.
type handler interface {
	// Key is called if the handler is in forcus
	// and the window receives a keyboard event.
	key(*window, key.Event) bool

	// Mouse is called if the handler is in focus
	// and the window receives a mouse event.
	mouse(*window, mouse.Event) bool

	// DrawLast is called if the handler is in focus
	// while the window is redrawn.
	// It is always called after everything else on the window has been drawn.
	drawLast(scr screen.Screen, win screen.Window)
}

const (
	minFrameSize = 21 // px
	borderWidth  = 1  // px
)

var borderColor = color.Black

type window struct {
	id     string
	server *Server
	screen.Window
	image.Rectangle
	columns []*column
	xs      []float64
	inFocus handler
	p       image.Point
}

func newWindow(id string, s *Server, size image.Point) (*window, error) {
	win, err := s.screen.NewWindow(&screen.NewWindowOptions{
		Width:  size.X,
		Height: size.Y,
	})
	if err != nil {
		return nil, err
	}
	w := &window{
		id:        id,
		server:    s,
		Window:    win,
		Rectangle: image.Rect(0, 0, size.X, size.Y),
	}
	w.addColumn(0.0, newColumn())
	go w.events()
	return w, nil
}

type closeEvent struct{}

func (w *window) events() {
	events := make(chan interface{})
	go func() {
		for {
			e := w.NextEvent()
			if _, ok := e.(closeEvent); ok {
				close(events)
				return
			}
			events <- e
		}
	}()

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	var click int
	var redraw bool
	for {
		select {
		case <-ticker.C:
			if !redraw {
				break
			}
			w.draw(w.server.screen, w.Window)
			if w.inFocus != nil {
				w.inFocus.drawLast(w.server.screen, w.Window)
			}
			w.Publish()
			redraw = false

		case e, ok := <-events:
			if !ok {
				for _, c := range w.columns {
					c.close()
				}
				if f, ok := w.inFocus.(frame); ok {
					f.close()
				}
				w.Release()
				return
			}
			switch e := e.(type) {
			case func():
				e()
				redraw = true

			case lifecycle.Event:
				if e.To == lifecycle.StageDead {
					w.server.delWin(w.id)
				}

			case paint.Event:
				redraw = true

			case size.Event:
				w.setBounds(image.Rectangle{Max: e.Size()})

			case key.Event:
				if e.Direction == key.DirRelease {
					continue
				}
				if w.inFocus != nil && w.inFocus.key(w, e) {
					redraw = true
				}

			case mouse.Event:
				var dir mouse.Direction
				w.p, dir = image.Pt(int(e.X), int(e.Y)), e.Direction
				switch dir {
				case mouse.DirPress:
					click++
				case mouse.DirRelease:
					click--
				}
				if dir == mouse.DirNone && click == 0 {
					prev := w.inFocus
					w.inFocus = w.focus(w.p)
					if prev != w.inFocus {
						redraw = true
					}
				}
				if w.inFocus != nil {
					if w.inFocus.mouse(w, e) {
						redraw = true
					}
				}
				// After sending a press or release to the focus,
				// check whether it's still in focus.
				if dir != mouse.DirNone {
					prev := w.inFocus
					w.inFocus = w.focus(w.p)
					if prev != w.inFocus {
						redraw = true
					}
				}
			}
		}
	}
}

func (w *window) close() {
	w.Send(closeEvent{})
}

func (w *window) bounds() image.Rectangle { return w.Rectangle }

func (w *window) setBounds(bounds image.Rectangle) {
	w.Rectangle = bounds
	width := float64(bounds.Dx())
	for i := len(w.columns) - 1; i >= 0; i-- {
		c := w.columns[i]
		b := bounds
		if i > 0 {
			b.Min.X = bounds.Min.X + int(width*w.xs[i])
		}
		if i < len(w.columns)-1 {
			b.Max.X = w.columns[i+1].bounds().Min.X - borderWidth
		}
		c.setBounds(b)
	}
}

func (w *window) focus(p image.Point) handler {
	for _, c := range w.columns {
		if p.In(c.bounds()) {
			return c.focus(p)
		}
	}
	return nil
}

func (w *window) draw(scr screen.Screen, win screen.Window) {
	for i, c := range w.columns {
		c.draw(scr, win)
		if i == len(w.columns)-1 {
			continue
		}
		d := w.columns[i+1]
		b := w.bounds()
		b.Min.X = c.bounds().Max.X
		b.Max.X = d.bounds().Min.X
		win.Fill(b, borderColor, draw.Over)
	}
}

// AddFrame adds the frame to the last column of the window.
func (w *window) addFrame(f frame) {
	c := w.columns[len(w.columns)-1]
	y := minFrameSize
	if len(c.frames) > 1 {
		f := c.frames[len(c.frames)-1]
		b := f.bounds()
		y = b.Min.Y + b.Dy()/2
	}
	c.addFrame(float64(y)/float64(c.Dy()), f)
}

func (w *window) deleteFrame(f frame) {
	for _, c := range w.columns {
		for _, g := range c.frames {
			if g == f {
				c.removeFrame(f)
			}
		}
	}
	if h := f.(handler); h == w.inFocus {
		w.inFocus = w.focus(w.p)
	}
	f.close()
}

func (w *window) deleteColumn(c *column) {
	if w.removeColumn(c) {
		c.close()
	}
}

func (w *window) removeColumn(c *column) bool {
	if len(w.columns) < 2 {
		return false
	}
	i := columnIndex(w, c)
	if i < 0 {
		return false
	}
	w.columns = append(w.columns[:i], w.columns[i+1:]...)
	w.xs = append(w.xs[:i], w.xs[i+1:]...)
	w.setBounds(w.bounds())
	c.win = nil
	return true
}

func columnIndex(w *window, c *column) int {
	for i := range w.columns {
		if w.columns[i] == c {
			return i
		}
	}
	return -1
}

// AddCol adds a column to the window such that its left side at pixel xfrac*w.Dx().
// However, if the window has no columns, its left side is always at 0.0.
func (w *window) addColumn(xfrac float64, c *column) bool {
	if len(w.columns) == 0 {
		w.columns = []*column{c}
		w.xs = []float64{0.0}
		c.win = w
		c.setBounds(w.bounds())
		return true
	}
	if xfrac < 0.0 {
		xfrac = 0.0
	}
	if xfrac > 1.0 {
		xfrac = 1.0
	}
	x := int(float64(w.Dx()) * xfrac)
	i, d := columnAt(w, x)
	if i < 0 || d == nil {
		return false
	}
	if sz := x - borderWidth - d.Min.X; sz < minFrameSize {
		if !w.slideLeft(i, minFrameSize-sz) {
			x += minFrameSize - sz
		}
	}
	if sz := d.Max.X - x; sz < minFrameSize {
		if !w.slideRight(i, minFrameSize-sz) {
			return false
		}
	}

	w.columns = append(w.columns, nil)
	if i+2 < len(w.columns) {
		copy(w.columns[i+2:], w.columns[i+1:])
	}
	w.columns[i+1] = c

	w.xs = append(w.xs, 0)
	if i+2 < len(w.xs) {
		copy(w.xs[i+2:], w.xs[i+1:])
	}
	w.xs[i+1] = xfrac

	c.win = w
	w.setBounds(w.bounds())
	return true
}

func columnAt(w *window, x int) (i int, c *column) {
	for i, c = range w.columns {
		if c.Max.X > x {
			return i, c
		}
	}
	return -1, nil
}

func (w *window) slideLeft(i int, delta int) bool {
	if i <= 0 {
		return false
	}
	side := w.columns[i].Min.X - delta
	if sz := side - w.columns[i-1].Min.X; sz < minFrameSize {
		if !w.slideLeft(i-1, minFrameSize-sz) {
			return false
		}
	}
	w.setX(i, side)
	return true
}

func (w *window) slideRight(i int, delta int) bool {
	if i > len(w.columns)-2 {
		return false
	}
	side := w.columns[i].Max.X + delta
	if sz := w.columns[i+1].Max.X - borderWidth - side; sz < minFrameSize {
		if !w.slideRight(i+1, minFrameSize-sz) {
			return false
		}
	}
	w.setX(i+1, side)
	return true
}

func (w *window) setX(i int, x int) {
	if i <= 0 || i > len(w.columns)-1 {
		return
	}
	if min := w.columns[i-1].Min.X + minFrameSize; x < min {
		x = min
	}
	if max := w.columns[i].Max.X - minFrameSize; x > max {
		x = max
	}
	w.xs[i] = float64(x) / float64(w.Dx())
	w.setBounds(w.bounds())
}

// A frame is a rectangular section of a win that can be attached to a column.
type frame interface {
	// Bounds returns the current bounds of the frame.
	bounds() image.Rectangle

	// SetBounds sets the bounds of the frame to the given rectangle.
	setBounds(image.Rectangle)

	// SetColumn sets the frame's column.
	setColumn(*column)

	// Focus returns the handler that is in focus at the given coordinate.
	// The upper-left of the frame is the Min point of its bounds.
	focus(image.Point) handler

	// Draw draws the frame to the window.
	draw(scr screen.Screen, win screen.Window)

	// Close closes the frame.
	// It is called by the containing object when that object has been removed.
	// Close should release the resources of the frame.
	// It should not remove the frame from its containing object,
	// because close is only intended to be called
	// after the frame has been removed from its container.
	close()
}

type column struct {
	win *window
	image.Rectangle
	frames []frame
	ys     []float64

	p      image.Point
	button mouse.Button
	origX  float64
}

// NewColumn returns a new column, with a body, but no window or bounds.
func newColumn() *column {
	c := new(column)
	c.addFrame(0, new(columnTag))
	return c
}

func (c *column) close() {
	for _, f := range c.frames {
		f.close()
	}
}

func (c *column) bounds() image.Rectangle { return c.Rectangle }

func (c *column) setBounds(bounds image.Rectangle) {
	c.Rectangle = bounds
	height := float64(bounds.Dy())
	for i := len(c.frames) - 1; i >= 0; i-- {
		f := c.frames[i]
		b := bounds
		if i > 0 {
			b.Min.Y = bounds.Min.Y + int(height*c.ys[i])
		}
		if i < len(c.frames)-1 {
			b.Max.Y = c.frames[i+1].bounds().Min.Y - borderWidth
		}
		f.setBounds(b)
	}
}

func (c *column) focus(p image.Point) handler {
	for _, f := range c.frames {
		if p.In(f.bounds()) {
			return f.focus(p)
		}
	}
	return nil
}

func (c *column) draw(scr screen.Screen, win screen.Window) {
	for i, f := range c.frames {
		f.draw(scr, win)
		if i == len(c.frames)-1 {
			continue
		}
		g := c.frames[i+1]
		b := c.bounds()
		b.Min.Y = f.bounds().Max.Y
		b.Max.Y = g.bounds().Min.Y
		win.Fill(b, borderColor, draw.Over)
	}
}

func (c *column) removeFrame(f frame) bool {
	if len(c.frames) == 1 {
		return false
	}
	i := frameIndex(c, f)
	if i < 0 {
		return false
	}
	c.frames = append(c.frames[:i], c.frames[i+1:]...)
	c.ys = append(c.ys[:i], c.ys[i+1:]...)
	c.setBounds(c.bounds())
	f.setColumn(nil)
	return true
}

func frameIndex(c *column, f frame) int {
	for i := range c.frames {
		if c.frames[i] == f {
			return i
		}
	}
	return -1
}

func (c *column) addFrame(yfrac float64, f frame) bool {
	if len(c.frames) == 0 {
		c.frames = []frame{f}
		c.ys = []float64{0.0}
		f.setColumn(c)
		f.setBounds(c.bounds())
		return true
	}
	if yfrac < 0.0 {
		yfrac = 0.0
	}
	if yfrac > 1.0 {
		yfrac = 1.0
	}
	y := int(yfrac * float64(c.Dy()))
	i, g := frameAt(c, y)
	if i < 0 || g == nil {
		return false
	}
	if sz := y - borderWidth - g.bounds().Min.Y; i > 0 && sz < minFrameSize {
		if !c.slideUp(i, minFrameSize-sz) {
			y += minFrameSize - sz
		}
	}
	if sz := g.bounds().Max.Y - y; sz < minFrameSize {
		if !c.slideDown(i, minFrameSize-sz) {
			return false
		}
	}

	c.frames = append(c.frames, nil)
	if i+2 < len(c.frames) {
		copy(c.frames[i+2:], c.frames[i+1:])
	}
	c.frames[i+1] = f

	c.ys = append(c.ys, 0)
	if i+2 < len(c.ys) {
		copy(c.ys[i+2:], c.ys[i+1:])
	}
	c.ys[i+1] = yfrac

	f.setColumn(c)
	c.setBounds(c.bounds())

	return true
}

func frameAt(c *column, y int) (i int, f frame) {
	for i, f = range c.frames {
		if f.bounds().Max.Y > y {
			return i, f
		}
	}
	return -1, nil
}

func (c *column) slideUp(i int, delta int) bool {
	if i <= 0 {
		return false
	}
	min := minFrameSize
	if i == 1 {
		min = 0 // min size of the 0th cell is 0.
	}
	side := c.frames[i].bounds().Min.Y - delta
	if sz := side - c.frames[i-1].bounds().Min.Y - borderWidth; sz < min {
		if !c.slideUp(i-1, min-sz) {
			return false
		}
	}
	c.setY(i, side)
	return true
}

func (c *column) slideDown(i int, delta int) bool {
	if i > len(c.frames)-2 {
		return false
	}
	side := c.frames[i].bounds().Max.Y + delta
	if sz := c.frames[i+1].bounds().Max.Y - borderWidth - side; sz < minFrameSize {
		if !c.slideDown(i+1, minFrameSize-sz) {
			return false
		}
	}
	c.setY(i+1, side)
	return true
}

func (c *column) setY(i int, y int) {
	if i <= 0 || i > len(c.frames)-1 {
		return
	}
	if min := c.frames[i-1].bounds().Min.Y + minFrameSize; i > 1 && y < min {
		y = min
	}
	if max := c.frames[i].bounds().Max.Y - minFrameSize; y > max {
		y = max
	}
	c.ys[i] = float64(y) / float64(c.Dy())
	c.setBounds(c.bounds())
}

type columnTag struct {
	col *column
	image.Rectangle

	p      image.Point
	button mouse.Button
	origX  float64
}

func (t *columnTag) close() {
	if t.col != nil && t.col.win == nil {
		t.col.close()
	}
	t.col = nil
}

func (t *columnTag) bounds() image.Rectangle     { return t.Rectangle }
func (t *columnTag) setBounds(b image.Rectangle) { t.Rectangle = b }
func (t *columnTag) setColumn(c *column)         { t.col = c }
func (t *columnTag) focus(image.Point) handler   { return t }

func (t *columnTag) draw(_ screen.Screen, win screen.Window) {
	bg := color.Gray16{0xF5F5}
	win.Fill(t.bounds(), bg, draw.Over)
}

func (t *columnTag) drawLast(scr screen.Screen, win screen.Window) {
	if t.col.win != nil {
		return
	}
	t.col.draw(scr, win)
	b := t.col.bounds()
	x0, x1 := b.Min.X, b.Max.X
	y0, y1 := b.Min.Y, b.Max.Y
	win.Fill(image.Rect(x0, y0-borderWidth, x1, y0), borderColor, draw.Over)
	win.Fill(image.Rect(x0-borderWidth, y0, x0, y1), borderColor, draw.Over)
	win.Fill(image.Rect(x0, y1, x1, y1+borderWidth), borderColor, draw.Over)
	win.Fill(image.Rect(x1, y0, x1+borderWidth, y1), borderColor, draw.Over)
}

func (*columnTag) key(*window, key.Event) bool { return false }

func (t *columnTag) mouse(w *window, event mouse.Event) bool {
	p := image.Pt(int(event.X), int(event.Y))

	switch event.Direction {
	case mouse.DirPress:
		if t.button == mouse.ButtonNone {
			t.p = p
			t.button = event.Button
			return false
		}
		// A second button was pressed while the first was held.
		// ColBody doesn't use chords; treat this as a release of the first.
		event.Button = t.button
		fallthrough

	case mouse.DirRelease:
		if event.Button != t.button {
			// It's not the button the cell considers pressed.
			// Ignore it.
			break
		}
		defer func() { t.button = mouse.ButtonNone }()

		switch t.button {
		case mouse.ButtonMiddle:
			w.deleteColumn(t.col)
			return true
		case mouse.ButtonLeft:
			if t.col.win != nil {
				return t.col.slideDown(0, minFrameSize)
			}
			if w.addColumn(float64(t.Min.X)/float64(w.Dx()), t.col) {
				return true
			}
			// It didn't fit; just put it back where it came from.
			if !w.addColumn(t.origX, t.col) {
				panic("can't put it back")
			}
			return true
		}

	case mouse.DirNone:
		if t.button == mouse.ButtonNone {
			break
		}
		switch t.button {
		case mouse.ButtonLeft:
			if t.col.win == nil {
				t.col.setBounds(t.col.Add(p.Sub(t.col.Min)))
				return true
			}
			dx := t.p.X - p.X
			dy := t.p.Y - p.Y
			if dx*dx+dy*dy > 100 {
				t.p = p
				i := columnIndex(w, t.col)
				if i < 0 {
					return false
				}
				t.origX = w.xs[i]
				w.removeColumn(t.col)
				return true
			}
		}
	}
	return false
}
