package lb

import (
	"sync/atomic"
	"time"
)

type LatencyHistogram struct {
	Buckets [8]atomic.Int64 // counts per bucket
	Sum     atomic.Int64    // microsecs for mean
	Count   atomic.Int64    // count for mean
}

var histBounds = [7]int64{100, 500, 1_000, 5_000, 10_000, 50_000, 100_000}
var histLowerBound = [8]int64{0, 100, 500, 1_000, 5_000, 10_000, 50_000, 100_000}
var histUpperBound = [8]int64{100, 500, 1_000, 5_000, 10_000, 50_000, 100_000, 500_000}

func (h *LatencyHistogram) Record(d time.Duration) {
	d_micro := d.Microseconds()
	h.Sum.Add(d_micro)
	h.Count.Add(1)
	for i, bound := range histBounds {
		if d_micro < bound {
			h.Buckets[i].Add(1)
			return
		}
	}
	h.Buckets[7].Add(1) // overflow: ≥ 100ms
}
