package v1alpha1

import "testing"

func TestCheckSpecIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled *bool
		want    bool
	}{
		{"nil defaults to true", nil, true},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := CheckSpec{Enabled: tt.enabled}
			if got := cs.IsEnabled(); got != tt.want {
				t.Errorf("CheckSpec.IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProfileCheckRefIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled *bool
		want    bool
	}{
		{"nil defaults to true", nil, true},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := ProfileCheckRef{Enabled: tt.enabled}
			if got := ref.IsEnabled(); got != tt.want {
				t.Errorf("ProfileCheckRef.IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProfileCheckRefIdentifier(t *testing.T) {
	tests := []struct {
		name         string
		refName      string
		gateCheckRef string
		want         string
	}{
		{"builtin by name", "dns", "", "dns"},
		{"dynamic by ref", "", "ingress-controller-ready", "dynamic:ingress-controller-ready"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := ProfileCheckRef{
				Name:         tt.refName,
				GateCheckRef: tt.gateCheckRef,
			}
			if got := ref.Identifier(); got != tt.want {
				t.Errorf("ProfileCheckRef.Identifier() = %q, want %q", got, tt.want)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}
