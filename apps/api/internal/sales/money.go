package sales

import (
	"fmt"
	"strconv"
	"strings"
)

// Amounts are handled as integer cents and rates as integer millionths, so no value ever
// passes through a float. Rounding is half up, the same as the POS.

// parseFixed reads a non-negative decimal string into an integer scaled by 10^decimals.
func parseFixed(v string, decimals int) (int64, error) {
	whole, frac, _ := strings.Cut(strings.TrimSpace(v), ".")
	if len(frac) > decimals {
		frac = frac[:decimals]
	}
	frac += strings.Repeat("0", decimals-len(frac))
	n, err := strconv.ParseInt(whole+frac, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid amount %q", v)
	}
	return n, nil
}

func parseCents(v string) (int64, error) { return parseFixed(v, 2) }

func formatCents(c int64) string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

// divRound divides rounding half up; num and den are non-negative.
func divRound(num, den int64) int64 { return (num + den/2) / den }

// percentOf applies a percentage expressed in hundredths (1500 = 15%).
func percentOf(cents, hundredths int64) int64 { return divRound(cents*hundredths, 10000) }

// usdToNIO converts USD cents with a rate in millionths of córdoba per dollar.
func usdToNIO(usdCents, rateMicros int64) int64 { return divRound(usdCents*rateMicros, 1_000_000) }

// allocate splits amount across weights proportionally; leftover cents go to the largest
// remainders, so the shares always add up to amount.
func allocate(amount int64, weights []int64) []int64 {
	shares := make([]int64, len(weights))
	var total int64
	for _, w := range weights {
		total += w
	}
	if total == 0 || amount == 0 {
		return shares
	}
	remainders := make([]int64, len(weights))
	var given int64
	for i, w := range weights {
		shares[i] = amount * w / total
		remainders[i] = amount * w % total
		given += shares[i]
	}
	for left := amount - given; left > 0; left-- {
		best := 0
		for i := range remainders {
			if remainders[i] > remainders[best] {
				best = i
			}
		}
		shares[best]++
		remainders[best] = -1
	}
	return shares
}
