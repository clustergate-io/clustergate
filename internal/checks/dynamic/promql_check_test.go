package dynamic

import "testing"

func TestCompareFloat64(t *testing.T) {
	tests := []struct {
		name      string
		actual    float64
		operator  string
		threshold float64
		want      bool
	}{
		// gte
		{"gte: greater", 5, "gte", 3, true},
		{"gte: equal", 3, "gte", 3, true},
		{"gte: less", 2, "gte", 3, false},
		// lte
		{"lte: less", 2, "lte", 3, true},
		{"lte: equal", 3, "lte", 3, true},
		{"lte: greater", 5, "lte", 3, false},
		// eq
		{"eq: equal", 3, "eq", 3, true},
		{"eq: not equal", 4, "eq", 3, false},
		// gt
		{"gt: greater", 5, "gt", 3, true},
		{"gt: equal", 3, "gt", 3, false},
		{"gt: less", 2, "gt", 3, false},
		// lt
		{"lt: less", 2, "lt", 3, true},
		{"lt: equal", 3, "lt", 3, false},
		{"lt: greater", 5, "lt", 3, false},
		// unknown
		{"unknown operator", 5, "unknown", 3, false},
		{"empty operator", 5, "", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareFloat64(tt.actual, tt.operator, tt.threshold)
			if got != tt.want {
				t.Errorf("compareFloat64(%v, %q, %v) = %v, want %v", tt.actual, tt.operator, tt.threshold, got, tt.want)
			}
		})
	}
}
