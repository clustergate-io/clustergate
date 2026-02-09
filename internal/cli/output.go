package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// FormatText writes a human-readable report to the writer.
func FormatText(w io.Writer, report *Report) {
	fmt.Fprintln(w, "CLUSTERGATE CHECK RESULTS")
	fmt.Fprintln(w, "=========================")
	fmt.Fprintln(w)

	for _, c := range report.Checks {
		marker := "[PASS]"
		if c.Status == "Failing" {
			marker = "[FAIL]"
		}
		fmt.Fprintf(w, "%s %s (%s/%s)\n", marker, c.Name, c.Category, c.Severity)
		fmt.Fprintf(w, "       %s\n", c.Message)
		fmt.Fprintln(w)
	}

	if len(report.Errors) > 0 {
		for _, e := range report.Errors {
			fmt.Fprintf(w, "[ERR]  %s\n", e.Name)
			fmt.Fprintf(w, "       %s\n", e.Error)
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintln(w, strings.Repeat("-", 25))

	if report.Failed > 0 {
		fmt.Fprintf(w, "Results: %d/%d passed, %d failed\n", report.Passed, report.Total, report.Failed)
	} else {
		fmt.Fprintf(w, "Results: %d/%d passed\n", report.Passed, report.Total)
	}

	fmt.Fprintf(w, "Cluster State: %s\n", report.State)
}

// FormatJSON writes the report as indented JSON to the writer.
func FormatJSON(w io.Writer, report *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
