package launcher

import "math"

const minimumSongSelectRowHeight = float64(1)

// variableHeightLayout stores row heights and prefix sums for a list whose
// exact row sizes become known as rows are rendered. A Fenwick tree lets the
// selector correct one measured row without rewriting every following row.
//
// The selector cannot use ImGui's regular list clipper because song sets have
// different numbers of difficulties and their title text can wrap. Keeping
// this index independent from ImGui also makes the range and anchor behavior
// testable without creating a graphics context.
type variableHeightLayout struct {
	heights []float64
	tree    []float64
	maxBit  int
}

// rebuild replaces the layout with estimated heights. The tree is built in
// linear time instead of applying one logarithmic update per row because this
// operation is reserved for a new search, sort, catalog, or width.
func (l *variableHeightLayout) rebuild(count int, estimate func(index int) float32) {
	if count <= 0 {
		l.heights = l.heights[:0]
		l.tree = l.tree[:0]
		l.maxBit = 0
		return
	}

	if cap(l.heights) < count {
		l.heights = make([]float64, count)
	} else {
		l.heights = l.heights[:count]
		clear(l.heights)
	}

	if cap(l.tree) < count+1 {
		l.tree = make([]float64, count+1)
	} else {
		l.tree = l.tree[:count+1]
		clear(l.tree)
	}

	for i := range l.heights {
		height := minimumSongSelectRowHeight
		if estimate != nil {
			height = sanitizeLayoutHeight(estimate(i))
		}

		l.heights[i] = height
		l.tree[i+1] += height
	}

	// Convert the point values in tree into Fenwick partial sums. This is
	// linear because every element contributes directly to its parent once.
	for i := 1; i < len(l.tree); i++ {
		parent := i + (i & -i)
		if parent < len(l.tree) {
			l.tree[parent] += l.tree[i]
		}
	}

	l.maxBit = 1
	for next := l.maxBit << 1; next <= count; next <<= 1 {
		l.maxBit = next
	}
}

func sanitizeLayoutHeight(height float32) float64 {
	value := float64(height)
	if math.IsNaN(value) || math.IsInf(value, 0) || value < minimumSongSelectRowHeight {
		return minimumSongSelectRowHeight
	}

	return value
}

func (l *variableHeightLayout) len() int {
	return len(l.heights)
}

// top returns the accumulated height of all rows before index.
func (l *variableHeightLayout) top(index int) float32 {
	if index <= 0 {
		return 0
	}
	if index > len(l.heights) {
		index = len(l.heights)
	}

	return float32(l.prefix(index))
}

func (l *variableHeightLayout) height(index int) float32 {
	if index < 0 || index >= len(l.heights) {
		return 0
	}

	return float32(l.heights[index])
}

func (l *variableHeightLayout) total() float32 {
	return l.top(len(l.heights))
}

// update changes one row and returns the height delta. The caller can use the
// delta to preserve the visible scroll anchor when the changed row is fully
// above the viewport.
func (l *variableHeightLayout) update(index int, height float32) float32 {
	if index < 0 || index >= len(l.heights) {
		return 0
	}

	next := sanitizeLayoutHeight(height)
	delta := next - l.heights[index]
	if delta == 0 {
		return 0
	}

	l.heights[index] = next
	for i := index + 1; i < len(l.tree); i += i & -i {
		l.tree[i] += delta
	}

	return float32(delta)
}

// visibleRange returns a conservative range containing every row intersecting
// the viewport plus one row on either side. The binary search is performed on
// prefix sums, so scrolling does not inspect rows outside the viewport.
func (l *variableHeightLayout) visibleRange(scrollY, viewportHeight float32) (int, int) {
	count := len(l.heights)
	if count == 0 {
		return 0, 0
	}

	if scrollY < 0 || math.IsNaN(float64(scrollY)) {
		scrollY = 0
	}
	if viewportHeight < 0 || math.IsNaN(float64(viewportHeight)) {
		viewportHeight = 0
	}

	start := l.firstRowAtOrAfter(float64(scrollY))
	end := l.firstRowAtOrAfter(float64(scrollY + viewportHeight))

	if start > 0 {
		start--
	}
	if end < count {
		end++
	}
	if end <= start {
		end = min(count, start+1)
	}

	return start, end
}

// firstRowAtOrAfter returns the first row whose start is not before offset.
// An offset exactly on a row boundary resolves to the row after that
// boundary, which keeps the range calculation deterministic at separators.
func (l *variableHeightLayout) firstRowAtOrAfter(offset float64) int {
	count := len(l.heights)
	if count == 0 || offset <= 0 {
		return 0
	}
	if offset >= l.prefix(count) {
		return count
	}

	index := 0
	accumulated := float64(0)
	for bit := l.maxBit; bit > 0; bit >>= 1 {
		next := index + bit
		if next <= count && accumulated+l.tree[next] <= offset {
			index = next
			accumulated += l.tree[next]
		}
	}

	return index
}

func (l *variableHeightLayout) prefix(end int) float64 {
	if end <= 0 {
		return 0
	}
	if end > len(l.heights) {
		end = len(l.heights)
	}

	total := float64(0)
	for i := end; i > 0; i -= i & -i {
		total += l.tree[i]
	}

	return total
}
