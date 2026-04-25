package skills

import "testing"

func TestNormalizeSQLDriver(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"postgres", "pgx", false},
		{"PostgreSQL", "pgx", false},
		{"pgx", "pgx", false},
		{"mysql", "mysql", false},
		{"sqlite", "", true},
	}
	for _, tt := range tests {
		got, err := normalizeSQLDriver(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("%q: want error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q: got %q want %q", tt.in, got, tt.want)
		}
	}
}
