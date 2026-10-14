package translate

// EstimateTokens is a cheap token estimate (~4 bytes per token) used for balance
// pre-checks, count_tokens and as a fallback when upstream usage is missing.
func EstimateTokens(n int) int {
	if n <= 0 {
		return 0
	}
	return (n + 3) / 4
}
