package burst

import "math"

type Bucket struct{
	Count int64
	Sum   int64
}

type Histogram struct {
	b    [][]Bucket
	max  int64
	min  int64
	r    int
	size int
	n    int
}

func newHistogram(rb int) (ret Histogram) {
	// us, 10us, 100us, 1ms, 10ms, 100ms, 1s, 10s, 100s, 100s+
	initSize := 10
	ret.r = int(math.Pow10(rb))
	ret.size = initSize
	ret.b = make([][]Bucket, initSize)
	ret.min = math.MaxInt64
	ret.max = math.MinInt64
	for i := 0; i < initSize; i++ {
		ret.b[i] = make([]Bucket, ret.rowSize())
	}
	return
}

func (h *Histogram) rowSize() int {
	return h.r - h.r / 10
}

func NewHistogram() Histogram {
	return newHistogram(2)
}

func (h *Histogram) Clean() {
	for i := 0; i < h.size; i++ {
		for j := 0; j < h.rowSize(); j++ {
			h.b[i][j].Count++
		}
	}
}

func (h *Histogram) Add(n int64) {
	h.n++
	if n > h.max {
		h.max = n
	}
	if n < h.min {
		h.min = n
	}
	bi, oi := h.pos(n)
	for i := h.size; i <= bi; i++ {
		h.b = append(h.b, make([]Bucket, h.rowSize()))
	}
	h.b[bi][oi].Count++
	h.b[bi][oi].Sum += n
}

func (h *Histogram) pos(n int64) (int, int) {
	if n <= 0 {
		return 0, 0
	}
	bi := int(math.Log10(float64(n)))
	oi := int(n * int64(h.r / 10) / int64(math.Pow10(bi)) - int64(h.r / 10))
	return bi, oi
}

func (h *Histogram) Count() int {
	return h.n
}

func (h *Histogram) Max() int64 {
	return h.max
}

func (h *Histogram) Min() int64 {
	return h.min
}

func (h *Histogram) iterate(f func(int, int) bool) {
	i0, j0 := h.pos(h.min)
	i1, j1 := h.pos(h.max)
	i, j := i0, j0
	for {
		if (f(i, j)) {
			break
		}
		j++
		if j >= h.rowSize() {
			i++
			j = 0
		}
		if i > i1 || i == i1 && j > j1 {
			break
		}
	}
}

func (h *Histogram) Summery() []Bucket {
	ret := make([]Bucket, h.size)
	if h.n == 0 {
		return ret
	}
	h.iterate(func(i, j int) bool {
		ret[i].Count += h.b[i][j].Count
		ret[i].Sum += h.b[i][j].Sum
		return false
	})
	return ret
}

func (h *Histogram) Histogram(n int) []Bucket {
	ret := make([]Bucket, n)
	rest := int64(h.n)
	ns := int64(n)
	c := rest / ns
	rest -= c
	ri := 0
	h.iterate(func(i, j int) bool {
		if c == 0 {
			ns--
			if n == 0 || rest == 0 {
				return true
			}
			c = rest / ns
			rest -= c
			ri++
		}
		t := h.b[i][j]
		if ret[ri].Count + t.Count <= c {
			ret[ri].Count += t.Count
			ret[ri].Sum += t.Sum
			c -= t.Count
		} else {
			for t.Count > 0 {
				part := t.Sum * c / t.Count
				ret[ri].Sum += part
				t.Sum -= part
				t.Count -= c
				ret[ri].Count += c

				ns--
				if n == 0 || rest == 0 {
					return true
				}
				c = rest / ns
				rest -= c
				ri++
			}
		}
		return false
	})
	return ret
}
