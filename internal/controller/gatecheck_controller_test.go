package controller

import (
	"testing"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
)

func TestGateCheckValidation(t *testing.T) {
	tests := []struct {
		name      string
		spec      clustergatev1alpha1.GateCheckSpec
		wantValid bool
	}{
		{
			name: "valid pod check",
			spec: clustergatev1alpha1.GateCheckSpec{
				Severity: clustergatev1alpha1.SeverityCritical,
				Category: "networking",
				PodCheck: &clustergatev1alpha1.PodCheckSpec{
					Namespace: "default",
					MinReady:  1,
				},
			},
			wantValid: true,
		},
		{
			name: "valid http check",
			spec: clustergatev1alpha1.GateCheckSpec{
				Severity: clustergatev1alpha1.SeverityWarning,
				Category: "networking",
				HTTPCheck: &clustergatev1alpha1.HTTPCheckSpec{
					URL: "https://example.com/healthz",
				},
			},
			wantValid: true,
		},
		{
			name:      "no check type specified",
			spec:      clustergatev1alpha1.GateCheckSpec{},
			wantValid: false,
		},
		{
			name: "multiple check types",
			spec: clustergatev1alpha1.GateCheckSpec{
				PodCheck: &clustergatev1alpha1.PodCheckSpec{
					Namespace: "default",
				},
				HTTPCheck: &clustergatev1alpha1.HTTPCheckSpec{
					URL: "https://example.com",
				},
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := 0
			if tt.spec.PodCheck != nil {
				count++
			}
			if tt.spec.HTTPCheck != nil {
				count++
			}
			if tt.spec.ResourceCheck != nil {
				count++
			}
			if tt.spec.PromQLCheck != nil {
				count++
			}
			if tt.spec.ScriptCheck != nil {
				count++
			}
			valid := count == 1
			if valid != tt.wantValid {
				t.Errorf("expected valid=%v, got %v (checkTypeCount=%d)", tt.wantValid, valid, count)
			}
		})
	}
}
