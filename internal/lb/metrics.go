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

type LatencySnapshot struct {
	P50Us  int64 `json:"p50Us"`
	P95Us  int64 `json:"p95Us"`
	P99Us  int64 `json:"p99Us"`
	MeanUs int64 `json:"meanUs"`
}

var histBounds = [7]int64{100, 500, 1_000, 5_000, 10_000, 50_000, 100_000}
var histLowerBound = [8]int64{0, 100, 500, 1_000, 5_000, 10_000, 50_000, 100_000}
var histUpperBound = [8]int64{100, 500, 1_000, 5_000, 10_000, 50_000, 100_000, 500_000}

func (h *LatencyHistogram) Record(d time.Duration) {
	d_micro := d.Microseconds()
	h.Sum.Add(d_micro)
	for i, bound := range histBounds {
		if d_micro < bound {
			h.Buckets[i].Add(1)
			h.Count.Add(1)
			return
		}
	}
	h.Buckets[7].Add(1) // overflow: ≥ 100ms
	h.Count.Add(1)      // add count for overflow case
}

func (h *LatencyHistogram) Snapshot() LatencySnapshot {
	var counts [8]int64
	for i := range counts {
		counts[i] = h.Buckets[i].Load()
	}
	total := h.Count.Load()
	if total == 0 {
		return LatencySnapshot{}
	}
	sum := h.Sum.Load()
	return LatencySnapshot{
		P50Us:  estimatePercentile(counts, total, 50),
		P95Us:  estimatePercentile(counts, total, 95),
		P99Us:  estimatePercentile(counts, total, 99),
		MeanUs: sum / total,
	}
}

func estimatePercentile(counts [8]int64, total int64, pct int64) int64 {
	target := (total * pct) / 100
	var cumulative int64
	for i, count := range counts {
		cumulative += count
		if cumulative >= target {
			prev := cumulative - count
			fraction := float64(target-prev) / float64(count)
			return histLowerBound[i] + int64(fraction*float64(histUpperBound[i]-histLowerBound[i]))
		}
	}
	return histUpperBound[len(counts)-1]
}
