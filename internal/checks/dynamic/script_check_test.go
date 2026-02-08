package dynamic

import (
	"context"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
)

func TestTruncateLog(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"too long", "hello world", 5, "hello...(truncated)"},
		{"empty", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateLog(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateLog(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestExecuteScriptCheck_JobCreation(t *testing.T) {
	cs := kubefake.NewSimpleClientset()

	var jobCreated bool
	cs.PrependReactor("create", "jobs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction := action.(k8stesting.CreateAction)
		job := createAction.GetObject().(*batchv1.Job)

		// Verify job properties
		if job.Namespace != "test-ns" {
			t.Errorf("expected namespace test-ns, got %s", job.Namespace)
		}
		if job.Labels[labelManagedBy] != labelManagedByValue {
			t.Errorf("expected label %s=%s", labelManagedBy, labelManagedByValue)
		}
		if job.Labels[labelCheckName] != "my-check" {
			t.Errorf("expected label %s=my-check, got %s", labelCheckName, job.Labels[labelCheckName])
		}
		if *job.Spec.BackoffLimit != 0 {
			t.Errorf("expected backoffLimit 0, got %d", *job.Spec.BackoffLimit)
		}
		if *job.Spec.ActiveDeadlineSeconds != 1 {
			t.Errorf("expected activeDeadlineSeconds 1, got %d", *job.Spec.ActiveDeadlineSeconds)
		}
		container := job.Spec.Template.Spec.Containers[0]
		if container.Image != "busybox:latest" {
			t.Errorf("expected image busybox:latest, got %s", container.Image)
		}
		if container.Command[0] != "sh" || container.Command[1] != "-c" {
			t.Errorf("expected command [sh -c], got %v", container.Command)
		}
		if len(container.Args) != 1 || container.Args[0] != "echo hello" {
			t.Errorf("expected args [echo hello], got %v", container.Args)
		}
		if job.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyNever {
			t.Errorf("expected restartPolicy Never, got %s", job.Spec.Template.Spec.RestartPolicy)
		}

		// Set a generate name so Get works
		job.Name = "clustergate-my-check-abc123"
		jobCreated = true
		return false, nil, nil
	})

	// The Job will never complete since we're using a simple fake. The poll will timeout.
	// We use a very short timeout to make the test fast.
	timeoutSec := int32(1)
	spec := &clustergatev1alpha1.ScriptCheckSpec{
		Image:          "busybox:latest",
		Command:        []string{"sh", "-c"},
		Args:           []string{"echo hello"},
		TimeoutSeconds: &timeoutSec,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, _ := executeScriptCheck(ctx, cs, "test-ns", "my-check", spec)

	if !jobCreated {
		t.Fatal("expected Job to be created")
	}

	// With a 1s timeout the Job won't complete, so result should be not ready
	if result.Ready {
		t.Error("expected not ready for timed-out job")
	}
}

func TestExecuteScriptCheck_CustomTimeout(t *testing.T) {
	cs := kubefake.NewSimpleClientset()

	var capturedDeadline int64
	cs.PrependReactor("create", "jobs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction := action.(k8stesting.CreateAction)
		job := createAction.GetObject().(*batchv1.Job)
		capturedDeadline = *job.Spec.ActiveDeadlineSeconds
		job.Name = "clustergate-test-abc"
		return false, nil, nil
	})

	timeoutSec := int32(120)
	spec := &clustergatev1alpha1.ScriptCheckSpec{
		Image:          "alpine:latest",
		Command:        []string{"true"},
		TimeoutSeconds: &timeoutSec,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	executeScriptCheck(ctx, cs, "test-ns", "test", spec)

	if capturedDeadline != 120 {
		t.Errorf("expected activeDeadlineSeconds 120, got %d", capturedDeadline)
	}
}

func TestExecuteScriptCheck_DefaultTimeout(t *testing.T) {
	cs := kubefake.NewSimpleClientset()

	var capturedDeadline int64
	cs.PrependReactor("create", "jobs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction := action.(k8stesting.CreateAction)
		job := createAction.GetObject().(*batchv1.Job)
		capturedDeadline = *job.Spec.ActiveDeadlineSeconds
		job.Name = "clustergate-test-abc"
		return false, nil, nil
	})

	spec := &clustergatev1alpha1.ScriptCheckSpec{
		Image:   "alpine:latest",
		Command: []string{"true"},
		// No TimeoutSeconds — should default to 30
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	executeScriptCheck(ctx, cs, "test-ns", "test", spec)

	if capturedDeadline != int64(defaultScriptTimeout) {
		t.Errorf("expected activeDeadlineSeconds %d, got %d", defaultScriptTimeout, capturedDeadline)
	}
}

func TestExecuteScriptCheck_ServiceAccountName(t *testing.T) {
	cs := kubefake.NewSimpleClientset()

	var capturedSA string
	cs.PrependReactor("create", "jobs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction := action.(k8stesting.CreateAction)
		job := createAction.GetObject().(*batchv1.Job)
		capturedSA = job.Spec.Template.Spec.ServiceAccountName
		job.Name = "clustergate-test-abc"
		return false, nil, nil
	})

	timeoutSec := int32(1)
	spec := &clustergatev1alpha1.ScriptCheckSpec{
		Image:              "alpine:latest",
		Command:            []string{"true"},
		TimeoutSeconds:     &timeoutSec,
		ServiceAccountName: "my-sa",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	executeScriptCheck(ctx, cs, "test-ns", "test", spec)

	if capturedSA != "my-sa" {
		t.Errorf("expected serviceAccountName my-sa, got %s", capturedSA)
	}
}

func TestExecuteScriptCheck_EnvVars(t *testing.T) {
	cs := kubefake.NewSimpleClientset()

	var capturedEnv []corev1.EnvVar
	cs.PrependReactor("create", "jobs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction := action.(k8stesting.CreateAction)
		job := createAction.GetObject().(*batchv1.Job)
		capturedEnv = job.Spec.Template.Spec.Containers[0].Env
		job.Name = "clustergate-test-abc"
		return false, nil, nil
	})

	timeoutSec := int32(1)
	spec := &clustergatev1alpha1.ScriptCheckSpec{
		Image:          "alpine:latest",
		Command:        []string{"true"},
		TimeoutSeconds: &timeoutSec,
		Env: []corev1.EnvVar{
			{Name: "FOO", Value: "bar"},
			{Name: "BAZ", Value: "qux"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	executeScriptCheck(ctx, cs, "test-ns", "test", spec)

	if len(capturedEnv) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(capturedEnv))
	}
	if capturedEnv[0].Name != "FOO" || capturedEnv[0].Value != "bar" {
		t.Errorf("expected FOO=bar, got %s=%s", capturedEnv[0].Name, capturedEnv[0].Value)
	}
}

func TestPollJobCompletion_Success(t *testing.T) {
	completedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "test-ns",
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	cs := kubefake.NewSimpleClientset(completedJob)
	ctx := context.Background()

	result, err := pollJobCompletion(ctx, cs, "test-ns", "test-job", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ready {
		t.Error("expected ready=true for completed job")
	}
}

func TestPollJobCompletion_Failed(t *testing.T) {
	failedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "test-ns",
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
					Reason: "BackoffLimitExceeded",
				},
			},
		},
	}

	cs := kubefake.NewSimpleClientset(failedJob)
	ctx := context.Background()

	result, err := pollJobCompletion(ctx, cs, "test-ns", "test-job", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ready {
		t.Error("expected ready=false for failed job")
	}
	if result.reason != "BackoffLimitExceeded" {
		t.Errorf("expected reason BackoffLimitExceeded, got %s", result.reason)
	}
}

func TestPollJobCompletion_ContextCancelled(t *testing.T) {
	// Job that never completes — no conditions
	pendingJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "test-ns",
		},
	}

	cs := kubefake.NewSimpleClientset(pendingJob)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	result, err := pollJobCompletion(ctx, cs, "test-ns", "test-job", 30*time.Second)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
	if result.ready {
		t.Error("expected ready=false for cancelled context")
	}
}
