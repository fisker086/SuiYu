package agent

import "testing"

func TestStreamClientType(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"desktop", "desktop"},
		{"Desktop", "desktop"},
		{" web ", "web"},
		{"", "web"},
		{"mobile", "web"},
	}
	for _, tt := range tests {
		if got := streamClientType(tt.in); got != tt.want {
			t.Errorf("streamClientType(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
