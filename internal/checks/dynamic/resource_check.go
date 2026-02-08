package dynamic

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	preflightv1alpha1 "github.com/camcast3/platform-preflight/api/v1alpha1"
	"github.com/camcast3/platform-preflight/internal/checks"
)

func (e *Executor) executeResourceCheck(ctx context.Context, spec *preflightv1alpha1.ResourceCheckSpec) (checks.Result, error) {
	gv, err := schema.ParseGroupVersion(spec.APIVersion)
	if err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("invalid apiVersion %q: %v", spec.APIVersion, err),
		}, nil
	}
	gvk := gv.WithKind(spec.Kind)

	var resources []unstructured.Unstructured

	if spec.Name != "" {
		// Fetch a single named resource
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		key := types.NamespacedName{
			Namespace: spec.Namespace,
			Name:      spec.Name,
		}
		if err := e.client.Get(ctx, key, obj); err != nil {
			return checks.Result{
				Ready:   false,
				Message: fmt.Sprintf("resource %s/%s not found: %v", spec.Kind, spec.Name, err),
				Details: map[string]string{
					"apiVersion": spec.APIVersion,
					"kind":       spec.Kind,
					"name":       spec.Name,
					"namespace":  spec.Namespace,
				},
			}, nil
		}
		resources = append(resources, *obj)
	} else if spec.LabelSelector != nil {
		// Fetch resources matching label selector
		selector, err := metav1.LabelSelectorAsSelector(spec.LabelSelector)
		if err != nil {
			return checks.Result{}, fmt.Errorf("invalid label selector: %w", err)
		}

		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)
		opts := []client.ListOption{
			client.MatchingLabelsSelector{Selector: selector},
		}
		if spec.Namespace != "" {
			opts = append(opts, client.InNamespace(spec.Namespace))
		}
		if err := e.client.List(ctx, list, opts...); err != nil {
			return checks.Result{
				Ready:   false,
				Message: fmt.Sprintf("failed to list %s resources: %v", spec.Kind, err),
			}, nil
		}
		resources = list.Items
	} else {
		return checks.Result{
			Ready:   false,
			Message: "either name or labelSelector must be specified",
		}, nil
	}

	if len(resources) == 0 {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("no %s resources found", spec.Kind),
			Details: map[string]string{
				"apiVersion": spec.APIVersion,
				"kind":       spec.Kind,
				"namespace":  spec.Namespace,
			},
		}, nil
	}

	// Check conditions on each resource
	var failMessages []string
	for _, res := range resources {
		resName := res.GetName()
		conditions, found, err := unstructured.NestedSlice(res.Object, "status", "conditions")
		if err != nil || !found {
			failMessages = append(failMessages, fmt.Sprintf("%s: no conditions found", resName))
			continue
		}

		for _, expectedCond := range spec.Conditions {
			matched := false
			for _, c := range conditions {
				cond, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				condType, _ := cond["type"].(string)
				condStatus, _ := cond["status"].(string)
				if condType == expectedCond.Type && condStatus == expectedCond.Status {
					matched = true
					break
				}
			}
			if !matched {
				failMessages = append(failMessages, fmt.Sprintf("%s: condition %s != %s", resName, expectedCond.Type, expectedCond.Status))
			}
		}
	}

	details := map[string]string{
		"apiVersion":    spec.APIVersion,
		"kind":          spec.Kind,
		"namespace":     spec.Namespace,
		"resourceCount": fmt.Sprintf("%d", len(resources)),
	}

	if len(failMessages) > 0 {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("condition check failed: %s", strings.Join(failMessages, "; ")),
			Details: details,
		}, nil
	}

	return checks.Result{
		Ready:   true,
		Message: fmt.Sprintf("all %d %s resources have expected conditions", len(resources), spec.Kind),
		Details: details,
	}, nil
}
