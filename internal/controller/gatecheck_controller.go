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

// GateCheckReconciler reconciles a GateCheck object.
// It validates the spec and updates status conditions.
type GateCheckReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=clustergate.io,resources=gatechecks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=clustergate.io,resources=gatechecks/status,verbs=get;update;patch

func (r *GateCheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var gateCheck clustergatev1alpha1.GateCheck
	if err := r.Get(ctx, req.NamespacedName, &gateCheck); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.V(1).Info("reconciling GateCheck", "name", gateCheck.Name)

	// Validate that exactly one check type is specified.
	checkTypeCount := 0
	if gateCheck.Spec.PodCheck != nil {
		checkTypeCount++
	}
	if gateCheck.Spec.HTTPCheck != nil {
		checkTypeCount++
	}
	if gateCheck.Spec.ResourceCheck != nil {
		checkTypeCount++
	}
	if gateCheck.Spec.PromQLCheck != nil {
		checkTypeCount++
	}
	if gateCheck.Spec.ScriptCheck != nil {
		checkTypeCount++
	}

	condition := metav1.Condition{
		Type:               "Valid",
		ObservedGeneration: gateCheck.Generation,
	}

	if checkTypeCount == 1 {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "SpecValid"
		condition.Message = "GateCheck spec is valid"
	} else if checkTypeCount == 0 {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "NoCheckType"
		condition.Message = "Exactly one check type must be specified"
	} else {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "MultipleCheckTypes"
		condition.Message = "Exactly one check type must be specified, found multiple"
	}

	meta.SetStatusCondition(&gateCheck.Status.Conditions, condition)

	if err := r.Status().Update(ctx, &gateCheck); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GateCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clustergatev1alpha1.GateCheck{}).
		Complete(r)
}
