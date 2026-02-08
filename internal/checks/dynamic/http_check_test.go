package dynamic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
)

func TestHTTPCheck_Returns200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		HTTPCheck: &clustergatev1alpha1.HTTPCheckSpec{
			URL: srv.URL,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected ready=true, got false: %s", result.Message)
	}
}

func TestHTTPCheck_Returns500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		HTTPCheck: &clustergatev1alpha1.HTTPCheckSpec{
			URL: srv.URL,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false for 500 response")
	}
}

func TestHTTPCheck_CustomExpectedCodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated) // 201
	}))
	defer srv.Close()

	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		HTTPCheck: &clustergatev1alpha1.HTTPCheckSpec{
			URL:                 srv.URL,
			ExpectedStatusCodes: []int{200, 201},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected ready=true for 201 with expected codes [200,201]: %s", result.Message)
	}
}

func TestHTTPCheck_CustomHeaders(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	_, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		HTTPCheck: &clustergatev1alpha1.HTTPCheckSpec{
			URL:     srv.URL,
			Headers: map[string]string{"Authorization": "Bearer token123"},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedAuth != "Bearer token123" {
		t.Errorf("expected Authorization header = %q, got %q", "Bearer token123", receivedAuth)
	}
}

func TestHTTPCheck_DefaultMethodIsGET(t *testing.T) {
	var receivedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	_, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		HTTPCheck: &clustergatev1alpha1.HTTPCheckSpec{
			URL: srv.URL,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedMethod != http.MethodGet {
		t.Errorf("expected method = %q, got %q", http.MethodGet, receivedMethod)
	}
}

func TestHTTPCheck_InvalidURL(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		HTTPCheck: &clustergatev1alpha1.HTTPCheckSpec{
			URL: "://not-a-valid-url",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false for invalid URL")
	}
}

// Full PromQL tests with httptest mock

func promQLServer(t *testing.T, statusCode int, response interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}))
}

func TestPromQLCheck_ResultCountPassing(t *testing.T) {
	resp := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": "vector",
			"result": []interface{}{
				map[string]interface{}{"metric": map[string]string{"job": "etcd"}, "value": []interface{}{1.0, "1"}},
				map[string]interface{}{"metric": map[string]string{"job": "etcd"}, "value": []interface{}{1.0, "1"}},
				map[string]interface{}{"metric": map[string]string{"job": "etcd"}, "value": []interface{}{1.0, "1"}},
			},
		},
	}
	srv := promQLServer(t, 200, resp)
	defer srv.Close()

	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		PromQLCheck: &clustergatev1alpha1.PromQLCheckSpec{
			Endpoint: srv.URL,
			Query:    `up{job="etcd"} == 1`,
			Condition: clustergatev1alpha1.PromQLCondition{
				Type:      "resultCount",
				Operator:  "gte",
				Threshold: 3,
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected ready=true: %s", result.Message)
	}
}

func TestPromQLCheck_ResultCountFailing(t *testing.T) {
	resp := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": "vector",
			"result": []interface{}{
				map[string]interface{}{"metric": map[string]string{}, "value": []interface{}{1.0, "1"}},
				map[string]interface{}{"metric": map[string]string{}, "value": []interface{}{1.0, "1"}},
			},
		},
	}
	srv := promQLServer(t, 200, resp)
	defer srv.Close()

	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		PromQLCheck: &clustergatev1alpha1.PromQLCheckSpec{
			Endpoint: srv.URL,
			Query:    "up == 1",
			Condition: clustergatev1alpha1.PromQLCondition{
				Type:      "resultCount",
				Operator:  "gte",
				Threshold: 3,
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false: only 2 results but need >= 3")
	}
}

func TestPromQLCheck_ValuePassing(t *testing.T) {
	resp := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": "vector",
			"result": []interface{}{
				map[string]interface{}{"metric": map[string]string{}, "value": []interface{}{1.0, "0.001"}},
				map[string]interface{}{"metric": map[string]string{}, "value": []interface{}{1.0, "0.005"}},
			},
		},
	}
	srv := promQLServer(t, 200, resp)
	defer srv.Close()

	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		PromQLCheck: &clustergatev1alpha1.PromQLCheckSpec{
			Endpoint: srv.URL,
			Query:    "error_rate",
			Condition: clustergatev1alpha1.PromQLCondition{
				Type:      "value",
				Operator:  "lt",
				Threshold: 0.01,
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Errorf("expected ready=true: all values < 0.01: %s", result.Message)
	}
}

func TestPromQLCheck_ValueFailing(t *testing.T) {
	resp := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": "vector",
			"result": []interface{}{
				map[string]interface{}{"metric": map[string]string{}, "value": []interface{}{1.0, "0.001"}},
				map[string]interface{}{"metric": map[string]string{}, "value": []interface{}{1.0, "0.05"}},
			},
		},
	}
	srv := promQLServer(t, 200, resp)
	defer srv.Close()

	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		PromQLCheck: &clustergatev1alpha1.PromQLCheckSpec{
			Endpoint: srv.URL,
			Query:    "error_rate",
			Condition: clustergatev1alpha1.PromQLCondition{
				Type:      "value",
				Operator:  "lt",
				Threshold: 0.01,
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false: one value (0.05) exceeds 0.01")
	}
}

func TestPromQLCheck_NoResults(t *testing.T) {
	resp := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": "vector",
			"result":     []interface{}{},
		},
	}
	srv := promQLServer(t, 200, resp)
	defer srv.Close()

	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		PromQLCheck: &clustergatev1alpha1.PromQLCheckSpec{
			Endpoint: srv.URL,
			Query:    "up",
			Condition: clustergatev1alpha1.PromQLCondition{
				Type:      "value",
				Operator:  "gte",
				Threshold: 1,
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false for empty result set with value condition")
	}
}

func TestPromQLCheck_PrometheusHTTPError(t *testing.T) {
	srv := promQLServer(t, 500, map[string]string{"error": "internal error"})
	defer srv.Close()

	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		PromQLCheck: &clustergatev1alpha1.PromQLCheckSpec{
			Endpoint: srv.URL,
			Query:    "up",
			Condition: clustergatev1alpha1.PromQLCondition{
				Type:      "resultCount",
				Operator:  "gte",
				Threshold: 1,
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false for Prometheus HTTP 500")
	}
}

func TestPromQLCheck_PrometheusQueryError(t *testing.T) {
	resp := map[string]interface{}{
		"status":    "error",
		"errorType": "bad_data",
		"error":     "invalid query",
	}
	srv := promQLServer(t, 200, resp)
	defer srv.Close()

	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		PromQLCheck: &clustergatev1alpha1.PromQLCheckSpec{
			Endpoint: srv.URL,
			Query:    "invalid{",
			Condition: clustergatev1alpha1.PromQLCondition{
				Type:      "resultCount",
				Operator:  "gte",
				Threshold: 1,
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false for Prometheus query error")
	}
}

func TestPromQLCheck_UnknownConditionType(t *testing.T) {
	resp := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": "vector",
			"result": []interface{}{
				map[string]interface{}{"metric": map[string]string{}, "value": []interface{}{1.0, "1"}},
			},
		},
	}
	srv := promQLServer(t, 200, resp)
	defer srv.Close()

	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	result, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{
		PromQLCheck: &clustergatev1alpha1.PromQLCheckSpec{
			Endpoint: srv.URL,
			Query:    "up",
			Condition: clustergatev1alpha1.PromQLCondition{
				Type:      "invalid_type",
				Operator:  "gte",
				Threshold: 1,
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false for unknown condition type")
	}
}

func TestExecutor_NoCheckType(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(dynamicTestScheme()).Build()
	executor := newTestExecutor(c)
	_, err := executor.Execute(context.Background(), "test", clustergatev1alpha1.GateCheckSpec{})

	if err == nil {
		t.Error("expected error when no check type specified")
	}
}
