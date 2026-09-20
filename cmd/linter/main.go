// Command linter checks for panic and process termination outside main.
package main

import "golang.org/x/tools/go/analysis/singlechecker"

func main() {
	singlechecker.Main(analyzer)
}
