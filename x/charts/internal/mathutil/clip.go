package mathutil

// ClipLineToRect clips a line segment (x1,y1)-(x2,y2) to a rectangle.
// Returns the clipped segment and false if the line is entirely outside.
// Uses Cohen-Sutherland algorithm.
func ClipLineToRect(x1, y1, x2, y2, rxMin, ryMin, rxMax, ryMax float32) (cx1, cy1, cx2, cy2 float32, visible bool) {
	const (
		inside = 0
		left   = 1
		right  = 2
		bottom = 4
		top    = 8
	)

	code := func(x, y float32) int {
		c := inside
		if x < rxMin {
			c |= left
		} else if x > rxMax {
			c |= right
		}
		if y < ryMin {
			c |= top
		} else if y > ryMax {
			c |= bottom
		}
		return c
	}

	c1 := code(x1, y1)
	c2 := code(x2, y2)

	for iterations := 0; iterations < 20; iterations++ {
		if c1 == 0 && c2 == 0 {
			return x1, y1, x2, y2, true
		}
		if c1&c2 != 0 {
			return 0, 0, 0, 0, false
		}

		cOut := c1
		if cOut == 0 {
			cOut = c2
		}

		var x, y float32
		dx := x2 - x1
		dy := y2 - y1

		switch {
		case cOut&top != 0:
			if dy == 0 {
				return 0, 0, 0, 0, false
			}
			x = x1 + dx*(ryMin-y1)/dy
			y = ryMin
		case cOut&bottom != 0:
			if dy == 0 {
				return 0, 0, 0, 0, false
			}
			x = x1 + dx*(ryMax-y1)/dy
			y = ryMax
		case cOut&right != 0:
			if dx == 0 {
				return 0, 0, 0, 0, false
			}
			y = y1 + dy*(rxMax-x1)/dx
			x = rxMax
		case cOut&left != 0:
			if dx == 0 {
				return 0, 0, 0, 0, false
			}
			y = y1 + dy*(rxMin-x1)/dx
			x = rxMin
		}

		if cOut == c1 {
			x1, y1 = x, y
			c1 = code(x1, y1)
		} else {
			x2, y2 = x, y
			c2 = code(x2, y2)
		}
	}

	// Iteration limit reached; discard the segment
	return 0, 0, 0, 0, false
}
