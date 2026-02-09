package dynamic

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
	"github.com/clustergate/clustergate/internal/checks"
)

const (
	defaultScriptTimeout = 30
	scriptPollInterval   = 2 * time.Second
	labelManagedBy       = "app.kubernetes.io/managed-by"
	labelManagedByValue  = "clustergate"
	labelCheckName       = "clustergate.io/check"
)

// executeScriptCheck deploys a Kubernetes Job, waits for completion, reads
// the pod logs, and interprets the exit code.  Exit 0 → Ready, non-zero → not Ready.
func executeScriptCheck(ctx context.Context, clientset kubernetes.Interface, namespace string, checkName string, spec *clustergatev1alpha1.ScriptCheckSpec) (checks.Result, error) {
	timeout := int64(defaultScriptTimeout)
	if spec.TimeoutSeconds != nil {
		timeout = int64(*spec.TimeoutSeconds)
	}

	var backoffLimit int32 = 0
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("clustergate-%s-", checkName),
			Namespace:    namespace,
			Labels: map[string]string{
				labelManagedBy: labelManagedByValue,
				labelCheckName: checkName,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoffLimit,
			ActiveDeadlineSeconds: &timeout,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						labelManagedBy: labelManagedByValue,
						labelCheckName: checkName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: spec.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:    "script",
							Image:   spec.Image,
							Command: spec.Command,
							Args:    spec.Args,
							Env:     spec.Env,
						},
					},
				},
			},
		},
	}

	// Create the Job.
	created, err := clientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return checks.Result{}, fmt.Errorf("failed to create script check job: %w", err)
	}

	jobName := created.Name

	// Ensure cleanup regardless of outcome.
	defer func() {
		propagation := metav1.DeletePropagationBackground
		_ = clientset.BatchV1().Jobs(namespace).Delete(context.Background(), jobName, metav1.DeleteOptions{
			PropagationPolicy: &propagation,
		})
	}()

	// Poll until Job completes or context times out.
	result, err := pollJobCompletion(ctx, clientset, namespace, jobName, time.Duration(timeout)*time.Second)
	if err != nil {
		return checks.Result{}, err
	}

	// Read logs from the Job's pod.
	logOutput, logErr := getJobPodLogs(ctx, clientset, namespace, jobName)
	if logErr != nil {
		// Non-fatal: include error in message but still return the check result.
		logOutput = fmt.Sprintf("(failed to read logs: %v)", logErr)
	}

	if result.ready {
		return checks.Result{
			Ready:   true,
			Message: fmt.Sprintf("script completed successfully: %s", truncateLog(logOutput, 500)),
		}, nil
	}

	return checks.Result{
		Ready:   false,
		Message: fmt.Sprintf("script failed (reason: %s): %s", result.reason, truncateLog(logOutput, 500)),
	}, nil
}

// jobResult holds the outcome of a completed Job.
type jobResult struct {
	ready  bool
	reason string
}

// pollJobCompletion waits for a Job to reach a terminal state.
func pollJobCompletion(ctx context.Context, clientset kubernetes.Interface, namespace, jobName string, timeout time.Duration) (jobResult, error) {
	deadline := time.After(timeout)
	ticker := time.NewTicker(scriptPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return jobResult{ready: false, reason: "context cancelled"}, ctx.Err()
		case <-deadline:
			return jobResult{ready: false, reason: "timeout"}, nil
		case <-ticker.C:
			job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
			if err != nil {
				return jobResult{}, fmt.Errorf("failed to get job %s: %w", jobName, err)
			}

			for _, cond := range job.Status.Conditions {
				if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
					return jobResult{ready: true}, nil
				}
				if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
					return jobResult{ready: false, reason: cond.Reason}, nil
				}
			}
		}
	}
}

// getJobPodLogs finds the pod created by the Job and returns its logs.
func getJobPodLogs(ctx context.Context, clientset kubernetes.Interface, namespace, jobName string) (string, error) {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods for job %s: %w", jobName, err)
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for job %s", jobName)
	}

	podName := pods.Items[0].Name
	logStream, err := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{}).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get logs for pod %s: %w", podName, err)
	}
	defer logStream.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, logStream); err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return buf.String(), nil
}

// truncateLog truncates a log string to the given maximum length.
func truncateLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}
