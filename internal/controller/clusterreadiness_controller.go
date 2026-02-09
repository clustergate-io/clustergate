package controller

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
	"github.com/clustergate/clustergate/internal/checks"
	"github.com/clustergate/clustergate/internal/checks/dynamic"
	"github.com/clustergate/clustergate/internal/metrics"
	"github.com/clustergate/clustergate/internal/server"
)

const (
	defaultInterval = 60 * time.Second
)

// ClusterReadinessReconciler reconciles a ClusterReadiness object.
type ClusterReadinessReconciler struct {
	client.Client
	ReadinessState  *server.ReadinessState
	DynamicExecutor *dynamic.Executor
}

// +kubebuilder:rbac:groups=clustergate.io,resources=clusterreadinesses,verbs=get;list;watch
// +kubebuilder:rbac:groups=clustergate.io,resources=clusterreadinesses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="*",resources="*",verbs=get;list
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=create;delete;get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:urls="/healthz",verbs=get
// +kubebuilder:rbac:urls="/healthz/*",verbs=get
// +kubebuilder:rbac:urls="/livez",verbs=get
// +kubebuilder:rbac:urls="/livez/*",verbs=get
// +kubebuilder:rbac:urls="/readyz",verbs=get
// +kubebuilder:rbac:urls="/readyz/*",verbs=get

func (r *ClusterReadinessReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the ClusterReadiness resource.
	var cr clustergatev1alpha1.ClusterReadiness
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		// CR deleted — clean up state.
		r.ReadinessState.Remove(req.Name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconciling ClusterReadiness", "name", cr.Name)

	// Determine default requeue interval.
	interval := defaultInterval
	if cr.Spec.Interval.Duration > 0 {
		interval = cr.Spec.Interval.Duration
	}

	// Resolve profiles + inline checks into a flat list.
	resolvedChecks, err := ResolveChecks(ctx, r.Client, cr.Spec, interval)
	if err != nil {
		logger.Error(err, "failed to resolve checks")
		// Set a ProfilesResolved=False condition
		meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
			Type:               "ProfilesResolved",
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "ResolutionFailed",
			Message:            fmt.Sprintf("failed to resolve profiles: %v", err),
		})
		if updateErr := r.Status().Update(ctx, &cr); updateErr != nil {
			logger.Error(updateErr, "failed to update status after resolution failure")
		}
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	// Set ProfilesResolved condition if profiles are used
	if len(cr.Spec.Profiles) > 0 {
		meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
			Type:               "ProfilesResolved",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "AllProfilesResolved",
			Message:            fmt.Sprintf("resolved %d checks from %d profiles", len(resolvedChecks), len(cr.Spec.Profiles)),
		})
	}

	// Determine which checks are due for execution based on per-check intervals.
	now := metav1.Now()

	// Flatten existing categories for scheduler lookup.
	var existingChecks []clustergatev1alpha1.CheckStatus
	existingCategoryLookup := make(map[string]string)
	for _, cat := range cr.Status.Categories {
		for _, c := range cat.Checks {
			existingChecks = append(existingChecks, c)
			existingCategoryLookup[c.Name] = cat.Category
		}
	}

	dueChecks, carriedStatuses, nextRequeue := CheckSchedule(resolvedChecks, existingChecks, now.Time)

	logger.Info("check scheduling",
		"total", len(resolvedChecks),
		"due", len(dueChecks),
		"carried", len(carriedStatuses),
		"nextRequeue", nextRequeue,
	)

	// Run only due checks concurrently.
	results := make([]checkResult, len(dueChecks))
	var wg sync.WaitGroup

	for i, rc := range dueChecks {
		wg.Add(1)
		go func(idx int, resolved ResolvedCheck) {
			defer wg.Done()

			// Resolve final severity and category
			sev, cat := ResolveSeverityAndCategory(resolved, ctx, r.Client)

			if resolved.IsBuiltin {
				r.runBuiltinCheck(ctx, idx, resolved, sev, cat, results)
			} else {
				r.runResolvedDynamicCheck(ctx, idx, resolved, sev, cat, results)
			}
		}(i, rc)
	}

	wg.Wait()

	// Build status from results (newly executed + carried forward).
	healthChecks := make(map[string]*server.CheckState, len(results)+len(carriedStatuses))

	// Aggregation counters
	summary := &clustergatev1alpha1.ReadinessSummary{}
	categoryMap := make(map[string]*categoryAgg)

	// Process newly executed check results
	for _, res := range results {
		ready := res.result.Ready
		message := res.result.Message
		if res.err != nil {
			ready = false
			message = fmt.Sprintf("check error: %v", res.err)
		}

		status := "Passing"
		if !ready {
			status = "Failing"
		}

		cs := clustergatev1alpha1.CheckStatus{
			Name:        res.name,
			Source:      res.source,
			Status:      status,
			Severity:    clustergatev1alpha1.Severity(res.severity),
			Message:     message,
			LastChecked: &now,
		}

		healthChecks[res.name] = &server.CheckState{
			Status:   status,
			Message:  message,
			Severity: res.severity,
			Category: res.category,
		}

		// Update metrics.
		readyVal := float64(0)
		if ready {
			readyVal = 1
		}
		metrics.CheckReady.WithLabelValues(res.name, req.Name, res.severity, res.category).Set(readyVal)
		metrics.CheckDuration.WithLabelValues(res.name, res.severity, res.category).Observe(res.duration.Seconds())

		aggregateCheck(summary, categoryMap, res.severity, res.category, ready)
		categoryMap[res.category].checks = append(categoryMap[res.category].checks, cs)
	}

	// Process carried-forward check statuses
	for _, cs := range carriedStatuses {
		cat := existingCategoryLookup[cs.Name]

		healthChecks[cs.Name] = &server.CheckState{
			Status:   cs.Status,
			Message:  cs.Message,
			Severity: string(cs.Severity),
			Category: cat,
		}

		ready := cs.Status == "Passing"
		aggregateCheck(summary, categoryMap, string(cs.Severity), cat, ready)
		categoryMap[cat].checks = append(categoryMap[cat].checks, cs)
	}

	// Build categories with nested checks
	categories := make([]clustergatev1alpha1.CategoryStatus, 0, len(categoryMap))
	for _, agg := range categoryMap {
		var catState string
		if agg.criticalFailing {
			catState = "Unhealthy"
		} else if agg.warningFailing {
			catState = "Degraded"
		} else {
			catState = "Healthy"
		}

		// Sort checks within category for deterministic output
		sort.Slice(agg.checks, func(i, j int) bool {
			return agg.checks[i].Name < agg.checks[j].Name
		})

		categories = append(categories, clustergatev1alpha1.CategoryStatus{
			Category: agg.category,
			State:    catState,
			Checks:   agg.checks,
			Total:    agg.total,
			Passing:  agg.passing,
			Failing:  agg.failing,
		})

		// Update category metrics
		catReadyVal := float64(0)
		if !agg.criticalFailing {
			catReadyVal = 1
		}
		metrics.CategoryReady.WithLabelValues(agg.category, req.Name).Set(catReadyVal)
	}
	// Sort for deterministic output
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Category < categories[j].Category
	})

	// Readiness is determined by critical checks only
	allCriticalReady := summary.CriticalTotal == summary.CriticalPassing

	// Compute the cluster health state:
	// Healthy = all checks passing
	// Degraded = all critical passing but warning checks failing
	// Unhealthy = any critical check failing
	var healthState clustergatev1alpha1.ClusterHealthState
	if !allCriticalReady {
		healthState = clustergatev1alpha1.ClusterUnhealthy
	} else if summary.WarningFailing > 0 {
		healthState = clustergatev1alpha1.ClusterDegraded
	} else {
		healthState = clustergatev1alpha1.ClusterHealthy
	}

	// Update overall metrics.
	clusterReadyVal := float64(0)
	if allCriticalReady {
		clusterReadyVal = 1
	}
	metrics.ClusterReady.WithLabelValues(req.Name).Set(clusterReadyVal)
	metrics.ClusterHealthState.WithLabelValues(req.Name, string(healthState)).Set(1)
	// Reset other state gauges
	for _, s := range []string{"Healthy", "Degraded", "Unhealthy"} {
		if s != string(healthState) {
			metrics.ClusterHealthState.WithLabelValues(req.Name, s).Set(0)
		}
	}

	// Build health server summary view
	healthSummary := &server.ReadinessSummaryView{
		Total:           summary.Total,
		Passing:         summary.Passing,
		Failing:         summary.Failing,
		CriticalTotal:   summary.CriticalTotal,
		CriticalPassing: summary.CriticalPassing,
		WarningFailing:  summary.WarningFailing,
	}
	healthCategorySummaries := make([]server.CategorySummaryView, len(categories))
	for i, cs := range categories {
		healthCategorySummaries[i] = server.CategorySummaryView{
			Category: cs.Category,
			State:    cs.State,
			Total:    cs.Total,
			Passing:  cs.Passing,
			Failing:  cs.Failing,
		}
	}

	// Update health server state.
	r.ReadinessState.Update(req.Name, string(healthState), healthChecks, healthSummary, healthCategorySummaries)

	// Update CR status.
	cr.Status.State = healthState
	cr.Status.LastChecked = &now
	cr.Status.Categories = categories
	cr.Status.Summary = summary

	if err := r.Status().Update(ctx, &cr); err != nil {
		logger.Error(err, "failed to update ClusterReadiness status")
		return ctrl.Result{}, err
	}

	logger.Info("reconciliation complete",
		"state", healthState,
		"total", summary.Total,
		"criticalPassing", summary.CriticalPassing,
		"criticalTotal", summary.CriticalTotal,
		"warningFailing", summary.WarningFailing,
		"checksExecuted", len(dueChecks),
		"checksCarried", len(carriedStatuses),
		"requeueAfter", nextRequeue,
	)
	return ctrl.Result{RequeueAfter: nextRequeue}, nil
}

// SetupWithManager sets up the controller with the Manager.
// Watches ClusterReadiness, GateProfile, and GateCheck for changes.
func (r *ClusterReadinessReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clustergatev1alpha1.ClusterReadiness{}).
		Watches(&clustergatev1alpha1.GateProfile{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				return r.enqueueAllClusterReadiness(ctx)
			},
		)).
		Watches(&clustergatev1alpha1.GateCheck{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				return r.enqueueAllClusterReadiness(ctx)
			},
		)).
		Complete(r)
}

// enqueueAllClusterReadiness returns reconcile requests for all ClusterReadiness CRs.
func (r *ClusterReadinessReconciler) enqueueAllClusterReadiness(ctx context.Context) []reconcile.Request {
	var list clustergatev1alpha1.ClusterReadinessList
	if err := r.List(ctx, &list); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, len(list.Items))
	for i, cr := range list.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{Name: cr.Name},
		}
	}
	return requests
}

// runBuiltinCheck executes a built-in check by name.
func (r *ClusterReadinessReconciler) runBuiltinCheck(ctx context.Context, idx int, resolved ResolvedCheck, sev, cat string, results []checkResult) {
	checker, ok := checks.Get(resolved.BuiltinName)
	if !ok {
		results[idx] = checkResult{
			name:     resolved.Identifier,
			severity: sev,
			category: cat,
			source:   resolved.Source,
			result: checks.Result{
				Ready:   false,
				Message: fmt.Sprintf("unknown check: %s", resolved.BuiltinName),
			},
		}
		return
	}

	start := time.Now()
	res, err := checker.Run(ctx, resolved.Config)
	duration := time.Since(start)

	results[idx] = checkResult{
		name:     resolved.Identifier,
		severity: sev,
		category: cat,
		source:   resolved.Source,
		result:   res,
		err:      err,
		duration: duration,
	}
}

// runResolvedDynamicCheck executes a dynamic check via the GateCheck CR.
func (r *ClusterReadinessReconciler) runResolvedDynamicCheck(ctx context.Context, idx int, resolved ResolvedCheck, sev, cat string, results []checkResult) {
	var gc clustergatev1alpha1.GateCheck
	if err := r.Get(ctx, types.NamespacedName{Name: resolved.GateCheckName}, &gc); err != nil {
		results[idx] = checkResult{
			name:     resolved.Identifier,
			severity: sev,
			category: cat,
			source:   resolved.Source,
			result: checks.Result{
				Ready:   false,
				Message: fmt.Sprintf("GateCheck CR not found: %s", resolved.GateCheckName),
			},
		}
		return
	}

	start := time.Now()
	res, err := r.DynamicExecutor.Execute(ctx, resolved.GateCheckName, gc.Spec)
	duration := time.Since(start)

	results[idx] = checkResult{
		name:     resolved.Identifier,
		severity: sev,
		category: cat,
		source:   resolved.Source,
		result:   res,
		err:      err,
		duration: duration,
	}
}

// checkResult holds the outcome of a single check execution.
type checkResult struct {
	name     string
	severity string
	category string
	source   string
	result   checks.Result
	err      error
	duration time.Duration
}

// categoryAgg is a helper for accumulating per-category statistics.
type categoryAgg struct {
	category        string
	criticalFailing bool
	warningFailing  bool
	total           int
	passing         int
	failing         int
	checks          []clustergatev1alpha1.CheckStatus
}

// aggregateCheck updates summary and category aggregation for a single check result.
func aggregateCheck(summary *clustergatev1alpha1.ReadinessSummary, categoryMap map[string]*categoryAgg, severity, category string, ready bool) {
	summary.Total++
	if ready {
		summary.Passing++
	} else {
		summary.Failing++
	}

	switch clustergatev1alpha1.Severity(severity) {
	case clustergatev1alpha1.SeverityCritical:
		summary.CriticalTotal++
		if ready {
			summary.CriticalPassing++
		}
	case clustergatev1alpha1.SeverityWarning:
		summary.WarningTotal++
		if !ready {
			summary.WarningFailing++
		}
	}

	agg, exists := categoryMap[category]
	if !exists {
		agg = &categoryAgg{category: category}
		categoryMap[category] = agg
	}
	agg.total++
	if ready {
		agg.passing++
	} else {
		agg.failing++
		if clustergatev1alpha1.Severity(severity) == clustergatev1alpha1.SeverityCritical {
			agg.criticalFailing = true
		} else if clustergatev1alpha1.Severity(severity) == clustergatev1alpha1.SeverityWarning {
			agg.warningFailing = true
		}
	}
}
