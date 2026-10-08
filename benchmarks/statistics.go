package main

import (
	"math/rand/v2"
	"slices"
)

type summary struct {
	Pairs         int     `json:"pairs"`
	LatticeMedian float64 `json:"latticeMedianMilliseconds"`
	AspectMedian  float64 `json:"aspectMedianMilliseconds"`
	Ratio         float64 `json:"ratio"`
	Lower         float64 `json:"pairedBootstrap95Lower"`
	Upper         float64 `json:"pairedBootstrap95Upper"`
	Passed        bool    `json:"passed"`
}

func median(values []float64) float64 {
	values = slices.Clone(values)
	slices.Sort(values)
	if len(values)%2 == 1 {
		return values[len(values)/2]
	}
	return (values[len(values)/2-1] + values[len(values)/2]) / 2
}

func summarize(pairs []pair) summary {
	lattice, aspect := make([]float64, len(pairs)), make([]float64, len(pairs))
	for i, p := range pairs {
		lattice[i], aspect[i] = float64(p.Lattice.Total)/1e6, float64(p.Aspect.Total)/1e6
	}
	r := summary{Pairs: len(pairs), LatticeMedian: median(lattice), AspectMedian: median(aspect)}
	r.Ratio = r.LatticeMedian / r.AspectMedian
	// Resample whole pairs, retaining machine-load correlation and backend order.
	random := rand.New(rand.NewPCG(0x6c617474696365, 0x62656e63686d6172))
	ratios := make([]float64, 10_000)
	l, a := make([]float64, len(pairs)), make([]float64, len(pairs))
	for i := range ratios {
		for j := range l {
			index := random.IntN(len(pairs))
			l[j], a[j] = lattice[index], aspect[index]
		}
		ratios[i] = median(l) / median(a)
	}
	slices.Sort(ratios)
	r.Lower, r.Upper = ratios[249], ratios[9749]
	r.Passed = len(pairs) == 30 && r.Ratio < 1 && r.Upper < 1
	return r
}
