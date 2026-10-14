// Package billing implements pricing math and the batched usage ledger.
package billing

import (
	"fmt"
	"math"
	"math/big"
	"strings"
)

// Nano is an amount of money in nano-USD (1e-9 USD). Using integers avoids
// floating point drift and still represents per-request fees as small as
// 0.000001 cents (= 10 nano-USD).
type Nano = int64

const (
	NanoPerUSD  Nano = 1_000_000_000
	NanoPerCent Nano = 10_000_000
	perMillion       = 1_000_000
)

// Price describes how a single request is charged.
type Price struct {
	InputPerM  Nano // per 1M input tokens
	OutputPerM Nano // per 1M output tokens
	RequestFee Nano // flat fee per request
}

func (p Price) IsFree() bool { return p.InputPerM == 0 && p.OutputPerM == 0 && p.RequestFee == 0 }

// Cost returns the charge for a request, rounding token costs up so that the
// platform never undercharges by fractions of a nano-dollar.
func (p Price) Cost(inputTokens, outputTokens int) Nano {
	return tokenCost(inputTokens, p.InputPerM) + tokenCost(outputTokens, p.OutputPerM) + p.RequestFee
}

func tokenCost(tokens int, perM Nano) Nano {
	if tokens <= 0 || perM <= 0 {
		return 0
	}
	t := int64(tokens)
	// Guard against overflow for absurd values by falling back to big math.
	if perM > 0 && t > math.MaxInt64/perM {
		r := new(big.Int).Mul(big.NewInt(t), big.NewInt(perM))
		r.Add(r, big.NewInt(perMillion-1))
		r.Quo(r, big.NewInt(perMillion))
		if !r.IsInt64() {
			return math.MaxInt64
		}
		return r.Int64()
	}
	return (t*perM + perMillion - 1) / perMillion
}

// ApplyMarkup applies a markup expressed in basis points (100 bps = 1%).
func ApplyMarkup(v Nano, bps int) Nano {
	if v <= 0 {
		return 0
	}
	if bps == 0 {
		return v
	}
	r := new(big.Int).Mul(big.NewInt(v), big.NewInt(int64(10_000+bps)))
	r.Add(r, big.NewInt(9_999))
	r.Quo(r, big.NewInt(10_000))
	return r.Int64()
}

// ParseUSD parses a decimal USD string (e.g. "0.000003") into nano-USD,
// rounding up any precision beyond 9 decimals.
func ParseUSD(s string) (Nano, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	if r.Sign() < 0 {
		// Some catalogs use -1 for "variable pricing"; treat as unknown.
		return 0, fmt.Errorf("negative amount %q", s)
	}
	r.Mul(r, new(big.Rat).SetInt64(NanoPerUSD))
	q := new(big.Int).Quo(r.Num(), r.Denom())
	if new(big.Int).Mul(q, r.Denom()).Cmp(r.Num()) != 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() {
		return 0, fmt.Errorf("amount out of range %q", s)
	}
	return q.Int64(), nil
}

// PerTokenUSDToPerM converts a per-token USD price string to nano-USD per 1M tokens.
func PerTokenUSDToPerM(s string) (Nano, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return 0, fmt.Errorf("invalid price %q", s)
	}
	if r.Sign() < 0 {
		return 0, fmt.Errorf("negative price %q", s)
	}
	r.Mul(r, new(big.Rat).SetInt64(NanoPerUSD*perMillion))
	q := new(big.Int).Quo(r.Num(), r.Denom())
	if new(big.Int).Mul(q, r.Denom()).Cmp(r.Num()) != 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() {
		return 0, fmt.Errorf("price out of range %q", s)
	}
	return q.Int64(), nil
}

// FormatUSD renders nano-USD as a decimal string with trailing zeros trimmed.
func FormatUSD(v Nano) string {
	neg := v < 0
	if neg {
		v = -v
	}
	whole := v / NanoPerUSD
	frac := v % NanoPerUSD
	s := fmt.Sprintf("%d", whole)
	if frac != 0 {
		s += strings.TrimRight(fmt.Sprintf(".%09d", frac), "0")
	}
	if neg {
		s = "-" + s
	}
	return s
}
