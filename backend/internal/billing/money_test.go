package billing

import "testing"

func TestPriceCost(t *testing.T) {
	// $3 / 1M input, $15 / 1M output, 0.000001 cents per request.
	p := Price{InputPerM: 3 * NanoPerUSD, OutputPerM: 15 * NanoPerUSD, RequestFee: 10}
	got := p.Cost(1000, 500)
	// 1000*3/1e6 = $0.003 ; 500*15/1e6 = $0.0075 ; + 10 nano
	want := int64(3_000_000 + 7_500_000 + 10)
	if got != want {
		t.Fatalf("Cost = %d, want %d", got, want)
	}
	if p.Cost(0, 0) != 10 {
		t.Fatalf("empty request should cost only the fee")
	}
}

func TestTokenCostRoundsUp(t *testing.T) {
	// 1 token at $0.0000001/M is a tiny fraction of a nano: still charges 1 nano.
	p := Price{InputPerM: 100}
	if got := p.Cost(1, 0); got != 1 {
		t.Fatalf("got %d, want 1", got)
	}
}

func TestParseUSD(t *testing.T) {
	cases := map[string]int64{
		"0":            0,
		"1":            NanoPerUSD,
		"0.000001":     1000,
		"0.00000001":   10, // 0.000001 cents
		"2.5":          2_500_000_000,
		"0.0000000001": 1, // rounds up
		"":             0,
	}
	for in, want := range cases {
		got, err := ParseUSD(in)
		if err != nil || got != want {
			t.Errorf("ParseUSD(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	if _, err := ParseUSD("-1"); err == nil {
		t.Error("negative should fail")
	}
	if _, err := ParseUSD("abc"); err == nil {
		t.Error("garbage should fail")
	}
}

func TestPerTokenToPerM(t *testing.T) {
	got, err := PerTokenUSDToPerM("0.000003") // $3 per 1M
	if err != nil || got != 3*NanoPerUSD {
		t.Fatalf("got %d %v", got, err)
	}
	got, _ = PerTokenUSDToPerM("0.0000000375") // $0.0375 per 1M
	if got != 37_500_000 {
		t.Fatalf("got %d", got)
	}
	if _, err := PerTokenUSDToPerM("-1"); err == nil {
		t.Fatal("variable pricing should error")
	}
}

func TestApplyMarkup(t *testing.T) {
	if got := ApplyMarkup(1000, 1000); got != 1100 {
		t.Fatalf("10%% markup: %d", got)
	}
	if got := ApplyMarkup(1, 1000); got != 2 { // rounds up
		t.Fatalf("rounding: %d", got)
	}
	if got := ApplyMarkup(1000, 0); got != 1000 {
		t.Fatalf("no markup: %d", got)
	}
}

func TestFormatUSD(t *testing.T) {
	cases := map[int64]string{0: "0", NanoPerUSD: "1", 1_500_000_000: "1.5", 10: "0.00000001", -2 * NanoPerUSD: "-2"}
	for in, want := range cases {
		if got := FormatUSD(in); got != want {
			t.Errorf("FormatUSD(%d) = %q want %q", in, got, want)
		}
	}
}
