package edgetts

import "testing"

// Tokens gerados pela referencia oficial do edge-tts (mesmo material, mesmo
// instante): o valor v2 precisa ser identico, byte a byte.
func TestGenerateSecMSGECV2MatchesReference(t *testing.T) {
	cases := []struct {
		timestamp float64
		expected  string
	}{
		{1765000000.0, "B538F7BBB3CD8361-E46801D7149FAB338E06D8EE425515989069F4E177D2D6E7B0E7B586BD9F1D86"},
		{1765000123.0, "B538F7BBB3CD8361-E46801D7149FAB338E06D8EE425515989069F4E177D2D6E7B0E7B586BD9F1D86"},
		{1765000300.0, "732D1B44B1009E72-210486F4C51FB6D2D9B68881AC7F31858CB2440C3CF1D7813C49B647E742EE81"},
	}
	for _, item := range cases {
		got := generateSecMSGECV2(item.timestamp)
		if got != item.expected {
			t.Fatalf("token v2 para %v: esperado %s, obtido %s", item.timestamp, item.expected, got)
		}
	}
}
