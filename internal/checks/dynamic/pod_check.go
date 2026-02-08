package dynamic

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	preflightv1alpha1 "github.com/camcast3/platform-preflight/api/v1alpha1"
	"github.com/camcast3/platform-preflight/internal/checks"
)

func (e *Executor) executePodCheck(ctx context.Context, spec *preflightv1alpha1.PodCheckSpec) (checks.Result, error) {
	selector, err := convertLabelSelector(spec.LabelSelector)
	if err != nil {
		return checks.Result{}, fmt.Errorf("invalid label selector: %w", err)
	}

	podList := &corev1.PodList{}
	if err := e.client.List(ctx, podList,
		client.InNamespace(spec.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("failed to list pods: %v", err),
		}, nil
	}

	readyCount := int32(0)
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning && isPodReady(&pod) {
			readyCount++
		}
	}

	details := map[string]string{
		"namespace": spec.Namespace,
		"totalPods": fmt.Sprintf("%d", len(podList.Items)),
		"readyPods": fmt.Sprintf("%d", readyCount),
		"minReady":  fmt.Sprintf("%d", spec.MinReady),
	}

	if readyCount >= spec.MinReady {
		return checks.Result{
			Ready:   true,
			Message: fmt.Sprintf("%d/%d pods ready (minimum %d)", readyCount, len(podList.Items), spec.MinReady),
			Details: details,
		}, nil
	}

	return checks.Result{
		Ready:   false,
		Message: fmt.Sprintf("only %d/%d pods ready, need at least %d", readyCount, len(podList.Items), spec.MinReady),
		Details: details,
	}, nil
}

func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func convertLabelSelector(ls *metav1.LabelSelector) (labels.Selector, error) {
	if ls == nil {
		return labels.Everything(), nil
	}
	return metav1.LabelSelectorAsSelector(ls)
}
