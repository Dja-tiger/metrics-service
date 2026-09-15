// Package buildinfo prints application build metadata.
package buildinfo

import "fmt"

// Print writes build metadata to stdout, replacing empty values with N/A.
func Print(version, date, commit string) {
	fmt.Printf("Build version: %s\nBuild date: %s\nBuild commit: %s\n",
		valueOrDefault(version), valueOrDefault(date), valueOrDefault(commit))
}

func valueOrDefault(value string) string {
	if value == "" {
		return "N/A"
	}
	return value
}
