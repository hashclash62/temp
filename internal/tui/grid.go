package tui

// computeGrid calculates the cell bounding box and inner ASCII frame size.
func computeGrid(peerCount, termW, termH int) (cols, boxW, boxH, innerW, innerH int) {
	if peerCount <= 0 {
		return 1, termW, termH, 40, 15
	}

	cols = 1
	switch {
	case peerCount == 1:
		cols = 1
	case peerCount <= 4:
		cols = 2
	default:
		cols = 3
	}

	rows := (peerCount + cols - 1) / cols

	// Calculate outer bounding box for each cell
	boxW = termW / cols
	boxH = termH / rows

	// Calculate max available inner dimensions
	// Width overhead: 2 for borders, 2 for horizontal padding = 4
	maxInnerW := boxW - 4
	// Height overhead: 2 for borders, 1 for nameLabel, 1 for empty line, 1 safety = 5
	maxInnerH := boxH - 5

	if maxInnerW < 10 {
		maxInnerW = 10
	}
	if maxInnerH < 5 {
		maxInnerH = 5
	}

	// For a landscape video feed (e.g. 4:3), and terminal chars being 2x taller than wide:
	// Real aspect ratio = (innerW * 1) / (innerH * 2) = 4 / 3
	// innerW / innerH = 8 / 3  =>  innerH = innerW * 3 / 8
	
	targetH := maxInnerW * 3 / 8
	targetW := maxInnerW

	if targetH > maxInnerH {
		// Constrained by height, so calculate width based on height
		targetH = maxInnerH
		targetW = targetH * 8 / 3
	}

	return cols, boxW, boxH, targetW, targetH
}
