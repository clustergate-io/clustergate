package dynamic

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	clustergatev1alpha1 "github.com/clustergate/clustergate/api/v1alpha1"
	"github.com/clustergate/clustergate/internal/checks"
)

func (e *Executor) executeHTTPCheck(ctx context.Context, spec *clustergatev1alpha1.HTTPCheckSpec) (checks.Result, error) {
	method := spec.Method
	if method == "" {
		method = http.MethodGet
	}

	timeout := 10 * time.Second
	if spec.TimeoutSeconds != nil {
		timeout = time.Duration(*spec.TimeoutSeconds) * time.Second
	}

	expectedCodes := spec.ExpectedStatusCodes
	if len(expectedCodes) == 0 {
		expectedCodes = []int{http.StatusOK}
	}

	httpClient := httpClientForSpec(spec.InsecureSkipTLSVerify, timeout)

	req, err := http.NewRequestWithContext(ctx, method, spec.URL, nil)
	if err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("failed to create request: %v", err),
		}, nil
	}

	for k, v := range spec.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("HTTP request failed: %v", err),
			Details: map[string]string{
				"url":          spec.URL,
				"method":       method,
				"responseTime": elapsed.String(),
			},
		}, nil
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	details := map[string]string{
		"url":          spec.URL,
		"method":       method,
		"statusCode":   fmt.Sprintf("%d", resp.StatusCode),
		"responseTime": elapsed.String(),
	}

	for _, code := range expectedCodes {
		if resp.StatusCode == code {
			return checks.Result{
				Ready:   true,
				Message: fmt.Sprintf("%s %s returned %d", method, spec.URL, resp.StatusCode),
				Details: details,
			}, nil
		}
	}

	expectedStr := make([]string, len(expectedCodes))
	for i, c := range expectedCodes {
		expectedStr[i] = fmt.Sprintf("%d", c)
	}

	return checks.Result{
		Ready:   false,
		Message: fmt.Sprintf("%s %s returned %d, expected one of [%s]", method, spec.URL, resp.StatusCode, strings.Join(expectedStr, ", ")),
		Details: details,
	}, nil
}
