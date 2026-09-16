// Package buildinfo prints application build metadata.
package buildinfo

import "fmt"

// Print writes build metadata to stdout. Callers supply defaults at declaration.
func Print(version, date, commit string) {
	fmt.Printf("Build version: %s\nBuild date: %s\nBuild commit: %s\n",
		version, date, commit)
}
