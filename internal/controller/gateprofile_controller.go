package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
)

// GateProfileReconciler reconciles a GateProfile object.
// It validates the spec and updates status conditions.
type GateProfileReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=clustergate.io,resources=gateprofiles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=clustergate.io,resources=gateprofiles/status,verbs=get;update;patch

func (r *GateProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var profile clustergatev1alpha1.GateProfile
	if err := r.Get(ctx, req.NamespacedName, &profile); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.V(1).Info("reconciling GateProfile", "name", profile.Name)

	// Validate that each check reference has exactly one of Name or GateCheckRef.
	condition := metav1.Condition{
		Type:               "Valid",
		ObservedGeneration: profile.Generation,
	}

	valid := true
	for i, check := range profile.Spec.Checks {
		if check.Name == "" && check.GateCheckRef == "" {
			condition.Status = metav1.ConditionFalse
			condition.Reason = "InvalidCheckRef"
			condition.Message = "check at index " + string(rune('0'+i)) + " must specify either name or gateCheckRef"
			valid = false
			break
		}
		if check.Name != "" && check.GateCheckRef != "" {
			condition.Status = metav1.ConditionFalse
			condition.Reason = "AmbiguousCheckRef"
			condition.Message = "check at index " + string(rune('0'+i)) + " must specify only one of name or gateCheckRef"
			valid = false
			break
		}
	}

	if valid {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "SpecValid"
		condition.Message = "GateProfile spec is valid"
	}

	meta.SetStatusCondition(&profile.Status.Conditions, condition)

	if err := r.Status().Update(ctx, &profile); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GateProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clustergatev1alpha1.GateProfile{}).
		Complete(r)
}
