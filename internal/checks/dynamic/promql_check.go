package dynamic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	preflightv1alpha1 "github.com/camcast3/platform-preflight/api/v1alpha1"
	"github.com/camcast3/platform-preflight/internal/checks"
)

// promQLResponse represents the Prometheus HTTP API response for instant queries.
type promQLResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string            `json:"resultType"`
		Result     []json.RawMessage `json:"result"`
	} `json:"data"`
	Error     string `json:"error,omitempty"`
	ErrorType string `json:"errorType,omitempty"`
}

// promQLSample represents a single vector result.
type promQLSample struct {
	Metric map[string]string `json:"metric"`
	Value  [2]interface{}    `json:"value"` // [timestamp, "value_string"]
}

func (e *Executor) executePromQLCheck(ctx context.Context, spec *preflightv1alpha1.PromQLCheckSpec) (checks.Result, error) {
	timeout := 10 * time.Second
	if spec.TimeoutSeconds != nil {
		timeout = time.Duration(*spec.TimeoutSeconds) * time.Second
	}

	httpClient := httpClientForSpec(false, timeout)

	// Build Prometheus query URL
	queryURL, err := url.Parse(spec.Endpoint)
	if err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("invalid Prometheus endpoint URL: %v", err),
		}, nil
	}
	queryURL.Path = "/api/v1/query"
	params := url.Values{}
	params.Set("query", spec.Query)
	queryURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("failed to create request: %v", err),
		}, nil
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("Prometheus query failed: %v", err),
			Details: map[string]string{
				"endpoint": spec.Endpoint,
				"query":    spec.Query,
			},
		}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("failed to read Prometheus response: %v", err),
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("Prometheus returned HTTP %d: %s", resp.StatusCode, string(body)),
			Details: map[string]string{
				"endpoint":   spec.Endpoint,
				"query":      spec.Query,
				"statusCode": fmt.Sprintf("%d", resp.StatusCode),
			},
		}, nil
	}

	var promResp promQLResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("failed to parse Prometheus response: %v", err),
		}, nil
	}

	if promResp.Status != "success" {
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("Prometheus query error: %s (%s)", promResp.Error, promResp.ErrorType),
			Details: map[string]string{
				"endpoint": spec.Endpoint,
				"query":    spec.Query,
			},
		}, nil
	}

	resultCount := len(promResp.Data.Result)

	details := map[string]string{
		"endpoint":    spec.Endpoint,
		"query":       spec.Query,
		"resultCount": fmt.Sprintf("%d", resultCount),
		"resultType":  promResp.Data.ResultType,
	}

	switch spec.Condition.Type {
	case "resultCount":
		pass := compareFloat64(float64(resultCount), spec.Condition.Operator, spec.Condition.Threshold)
		if pass {
			return checks.Result{
				Ready:   true,
				Message: fmt.Sprintf("query returned %d results (%s %s %.0f)", resultCount, "resultCount", spec.Condition.Operator, spec.Condition.Threshold),
				Details: details,
			}, nil
		}
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("query returned %d results, expected %s %.0f", resultCount, spec.Condition.Operator, spec.Condition.Threshold),
			Details: details,
		}, nil

	case "value":
		if resultCount == 0 {
			return checks.Result{
				Ready:   false,
				Message: "query returned no results to evaluate",
				Details: details,
			}, nil
		}

		// Parse sample values
		var failedValues []string
		allPass := true
		for _, raw := range promResp.Data.Result {
			var sample promQLSample
			if err := json.Unmarshal(raw, &sample); err != nil {
				continue
			}
			valStr, ok := sample.Value[1].(string)
			if !ok {
				continue
			}
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				continue
			}
			if !compareFloat64(val, spec.Condition.Operator, spec.Condition.Threshold) {
				allPass = false
				failedValues = append(failedValues, fmt.Sprintf("%.4f", val))
			}
		}

		if allPass {
			return checks.Result{
				Ready:   true,
				Message: fmt.Sprintf("all %d sample values satisfy %s %.4f", resultCount, spec.Condition.Operator, spec.Condition.Threshold),
				Details: details,
			}, nil
		}
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("%d values failed condition %s %.4f", len(failedValues), spec.Condition.Operator, spec.Condition.Threshold),
			Details: details,
		}, nil

	default:
		return checks.Result{
			Ready:   false,
			Message: fmt.Sprintf("unknown condition type: %s", spec.Condition.Type),
		}, nil
	}
}

// compareFloat64 evaluates a comparison between two float64 values.
func compareFloat64(actual float64, operator string, threshold float64) bool {
	switch operator {
	case "gte":
		return actual >= threshold
	case "lte":
		return actual <= threshold
	case "eq":
		return actual == threshold
	case "gt":
		return actual > threshold
	case "lt":
		return actual < threshold
	default:
		return false
	}
}
