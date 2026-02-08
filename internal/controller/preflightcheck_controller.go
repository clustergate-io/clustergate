package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	preflightv1alpha1 "github.com/camcast3/platform-preflight/api/v1alpha1"
)

// PreflightCheckReconciler validates PreflightCheck CRs and sets their status.
type PreflightCheckReconciler struct {
	client.Client
}

// +kubebuilder:rbac:groups=preflight.platform.io,resources=preflightchecks,verbs=get;list;watch
// +kubebuilder:rbac:groups=preflight.platform.io,resources=preflightchecks/status,verbs=get;update;patch

func (r *PreflightCheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var pc preflightv1alpha1.PreflightCheck
	if err := r.Get(ctx, req.NamespacedName, &pc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("validating PreflightCheck", "name", pc.Name)

	// Validate exactly one check type is set
	typeCount := 0
	var checkType string
	if pc.Spec.PodCheck != nil {
		typeCount++
		checkType = "podCheck"
	}
	if pc.Spec.HTTPCheck != nil {
		typeCount++
		checkType = "httpCheck"
	}
	if pc.Spec.ResourceCheck != nil {
		typeCount++
		checkType = "resourceCheck"
	}
	if pc.Spec.PromQLCheck != nil {
		typeCount++
		checkType = "promqlCheck"
	}
	if pc.Spec.ScriptCheck != nil {
		typeCount++
		checkType = "scriptCheck"
	}

	valid := true
	message := fmt.Sprintf("check definition is valid (type: %s)", checkType)

	if typeCount == 0 {
		valid = false
		message = "exactly one check type must be specified (podCheck, httpCheck, resourceCheck, promqlCheck, or scriptCheck)"
	} else if typeCount > 1 {
		valid = false
		message = "only one check type may be specified"
	}

	// Type-specific validation
	if valid {
		switch {
		case pc.Spec.PodCheck != nil:
			if pc.Spec.PodCheck.Namespace == "" {
				valid = false
				message = "podCheck.namespace is required"
			} else if pc.Spec.PodCheck.LabelSelector == nil {
				valid = false
				message = "podCheck.labelSelector is required"
			}
		case pc.Spec.HTTPCheck != nil:
			if pc.Spec.HTTPCheck.URL == "" {
				valid = false
				message = "httpCheck.url is required"
			}
		case pc.Spec.ResourceCheck != nil:
			if pc.Spec.ResourceCheck.APIVersion == "" || pc.Spec.ResourceCheck.Kind == "" {
				valid = false
				message = "resourceCheck.apiVersion and resourceCheck.kind are required"
			} else if pc.Spec.ResourceCheck.Name == "" && pc.Spec.ResourceCheck.LabelSelector == nil {
				valid = false
				message = "resourceCheck requires either name or labelSelector"
			} else if len(pc.Spec.ResourceCheck.Conditions) == 0 {
				valid = false
				message = "resourceCheck.conditions must have at least one entry"
			}
		case pc.Spec.PromQLCheck != nil:
			if pc.Spec.PromQLCheck.Endpoint == "" {
				valid = false
				message = "promqlCheck.endpoint is required"
			} else if pc.Spec.PromQLCheck.Query == "" {
				valid = false
				message = "promqlCheck.query is required"
			}
		case pc.Spec.ScriptCheck != nil:
			if pc.Spec.ScriptCheck.Image == "" {
				valid = false
				message = "scriptCheck.image is required"
			} else if len(pc.Spec.ScriptCheck.Command) == 0 {
				valid = false
				message = "scriptCheck.command must have at least one element"
			}
		}
	}

	// Update status
	pc.Status.Valid = valid
	pc.Status.Message = message

	// Set Valid condition
	now := metav1.Now()
	validCondition := metav1.Condition{
		Type:               "Valid",
		LastTransitionTime: now,
	}
	if valid {
		validCondition.Status = metav1.ConditionTrue
		validCondition.Reason = "ValidationPassed"
		validCondition.Message = message
	} else {
		validCondition.Status = metav1.ConditionFalse
		validCondition.Reason = "ValidationFailed"
		validCondition.Message = message
	}
	meta.SetStatusCondition(&pc.Status.Conditions, validCondition)

	if err := r.Status().Update(ctx, &pc); err != nil {
		logger.Error(err, "failed to update PreflightCheck status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PreflightCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&preflightv1alpha1.PreflightCheck{}).
		Complete(r)
}
