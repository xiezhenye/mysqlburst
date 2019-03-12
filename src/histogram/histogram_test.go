package histogram

import (
	"testing"
)

func TestHistogram_pos(t *testing.T) {
	hist := NewHistogram()
	i, j := hist.pos(0)
	if i != 0 || j != 0 {
		t.Errorf("%d %d %d", 0, i, j)
	}
	i, j = hist.pos(1)
	if i != 0 || j != 0 {
		t.Errorf("%d %d %d", 1, i, j)
	}
	i, j = hist.pos(9)
	if i != 0 || j != 80 {
		t.Errorf("%d %d %d", 9, i, j)
	}
	i, j = hist.pos(10)
	if i != 1 || j != 0 {
		t.Errorf("%d %d %d", 10, i, j)
	}
	i, j = hist.pos(100)
	if i != 2 || j != 0 {
		t.Errorf("%d %d %d", 100, i, j)
	}
	i, j = hist.pos(109)
	if i != 2 || j != 0 {
		t.Errorf("%d %d %d", 100, i, j)
	}
	i, j = hist.pos(110)
	if i != 2 || j != 1 {
		t.Errorf("%d %d %d", 100, i, j)
	}
	i, j = hist.pos(119)
	if i != 2 || j != 1 {
		t.Errorf("%d %d %d", 100, i, j)
	}
	i, j = hist.pos(199)
	if i != 2 || j != 9 {
		t.Errorf("%d %d %d", 100, i, j)
	}
	i, j = hist.pos(999)
	if i != 2 || j != 89 {
		t.Errorf("%d %d %d", 100, i, j)
	}

	hist = newHistogram(1)
	i, j = hist.pos(1)
	if i != 0 || j != 0 {
		t.Errorf("%d %d %d", 1, i, j)
	}
}

func TestHistogram_Summery(t *testing.T) {
	hist := NewHistogram()
	hist.Add(9999)
	hist.Add(1)
	hist.Add(123)
	hist.Add(321)
	hist.Add(555)
	if hist.Count() != 5 {
		t.Error("Count != 5")
	}
	if hist.Min() != 1 {
		t.Error("Min != 1")
	}
	if hist.Max() != 9999 {
		t.Error("Max != 9999")
	}
	s := hist.Summery()
	if s[0].Count != 1 || s[0].Sum != 1 {
		t.Errorf("p0[count, sum]!=%d, %d", 1, 1)
	}
	if s[1].Count != 0 || s[1].Sum != 0 {
		t.Errorf("p1[count, sum]!=%d, %d", 0, 0)
	}
	if s[2].Count != 3 || s[2].Sum != 999 {
		t.Errorf("p2[count, sum] %v !=%d, %d", s[2], 3, 999)
	}
	if s[3].Count != 1 || s[3].Sum != 9999 {
		t.Errorf("p3[count, sum]!=%d, %d", 1, 9999)
	}
}

func TestHistogram_Histogram(t *testing.T) {
	hist := NewHistogram()
	hist.Add(1)
	hist.Add(10)
	hist.Add(100)
	hist.Add(1000)
	h := hist.Histogram(4)
	if h[0].Count != 1 || h[0].Sum != 1 {
		t.Errorf("p0 %v != 1, 1\n", h[0])
	}
	if h[1].Count != 1 || h[1].Sum != 10 {
		t.Errorf("p1 %v != 1, 1\n", h[1])
	}
	if h[3].Count != 1 || h[3].Sum != 1000 {
		t.Errorf("p1 %v != 1, 1\n", h[3])
	}
	hist = NewHistogram()
	hist.Add(1)
	hist.Add(2)
	hist.Add(3)
	hist.Add(4)
	hist.Add(5)
	h = hist.Histogram(2)
	t.Logf("%v\n", h)
	h = hist.Histogram(10)
	t.Logf("%v\n", h)
}
