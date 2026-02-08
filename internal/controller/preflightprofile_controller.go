package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	preflightv1alpha1 "github.com/camcast3/platform-preflight/api/v1alpha1"
	"github.com/camcast3/platform-preflight/internal/checks"
)

// PreflightProfileReconciler validates PreflightProfile CRs and sets their status.
type PreflightProfileReconciler struct {
	client.Client
}

// +kubebuilder:rbac:groups=preflight.platform.io,resources=preflightprofiles,verbs=get;list;watch
// +kubebuilder:rbac:groups=preflight.platform.io,resources=preflightprofiles/status,verbs=get;update;patch

func (r *PreflightProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var pp preflightv1alpha1.PreflightProfile
	if err := r.Get(ctx, req.NamespacedName, &pp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("validating PreflightProfile", "name", pp.Name)

	valid := true
	message := ""
	checkCount := 0

	for _, ref := range pp.Spec.Checks {
		if !ref.IsEnabled() {
			continue
		}
		checkCount++

		// Validate exactly one of name or preflightCheckRef is set
		hasName := ref.Name != ""
		hasRef := ref.PreflightCheckRef != ""

		if !hasName && !hasRef {
			valid = false
			message = "each check must specify either name or preflightCheckRef"
			break
		}
		if hasName && hasRef {
			valid = false
			message = fmt.Sprintf("check cannot specify both name (%q) and preflightCheckRef (%q)", ref.Name, ref.PreflightCheckRef)
			break
		}

		// Validate built-in checks exist
		if hasName {
			if _, ok := checks.Get(ref.Name); !ok {
				valid = false
				message = fmt.Sprintf("built-in check %q not found", ref.Name)
				break
			}
		}

		// Validate PreflightCheck CRs exist
		if hasRef {
			var pc preflightv1alpha1.PreflightCheck
			if err := r.Get(ctx, types.NamespacedName{Name: ref.PreflightCheckRef}, &pc); err != nil {
				valid = false
				message = fmt.Sprintf("PreflightCheck %q not found", ref.PreflightCheckRef)
				break
			}
		}
	}

	if valid && checkCount == 0 {
		valid = false
		message = "profile must contain at least one enabled check"
	}

	if valid {
		message = fmt.Sprintf("profile is valid with %d checks", checkCount)
	}

	// Update status
	pp.Status.Valid = valid
	pp.Status.CheckCount = checkCount
	pp.Status.Message = message

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
	meta.SetStatusCondition(&pp.Status.Conditions, validCondition)

	if err := r.Status().Update(ctx, &pp); err != nil {
		logger.Error(err, "failed to update PreflightProfile status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PreflightProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&preflightv1alpha1.PreflightProfile{}).
		Complete(r)
}
