package controller

import (
	"testing"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
)

func TestGateProfileValidation(t *testing.T) {
	critical := clustergatev1alpha1.SeverityCritical

	tests := []struct {
		name      string
		checks    []clustergatev1alpha1.ProfileCheckRef
		wantValid bool
	}{
		{
			name: "valid builtin ref",
			checks: []clustergatev1alpha1.ProfileCheckRef{
				{Name: "dns", Severity: &critical},
			},
			wantValid: true,
		},
		{
			name: "valid dynamic ref",
			checks: []clustergatev1alpha1.ProfileCheckRef{
				{GateCheckRef: "istiod-ready", Severity: &critical},
			},
			wantValid: true,
		},
		{
			name: "empty check ref",
			checks: []clustergatev1alpha1.ProfileCheckRef{
				{},
			},
			wantValid: false,
		},
		{
			name: "ambiguous check ref",
			checks: []clustergatev1alpha1.ProfileCheckRef{
				{Name: "dns", GateCheckRef: "istiod-ready"},
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := true
			for _, check := range tt.checks {
				if check.Name == "" && check.GateCheckRef == "" {
					valid = false
					break
				}
				if check.Name != "" && check.GateCheckRef != "" {
					valid = false
					break
				}
			}
			if valid != tt.wantValid {
				t.Errorf("expected valid=%v, got %v", tt.wantValid, valid)
			}
		})
	}
}
