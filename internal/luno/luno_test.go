package luno

import "testing"

func TestValid(t *testing.T) {
	tests := []struct {
		name   string
		number string
		want   bool
	}{
		{"valid 16-digit card number", "4276380000000005", true},
		{"valid from spec", "12345678903", true},
		{"valid short", "79927398713", true},
		{"zero", "0", true},
		{"invalid checksum", "4276380000000000", false},
		{"empty", "", false},
		{"letters", "1234abcd", false},
		{"spaces", "1234 5678", false},
		{"minus sign", "-12345678903", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Valid(tt.number); got != tt.want {
				t.Errorf("Valid(%q) = %v, want %v", tt.number, got, tt.want)
			}
		})
	}
}
