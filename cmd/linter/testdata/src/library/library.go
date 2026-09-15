package library

import (
	logger "log"
	process "os"
)

func main() {
	logger.Fatal("failed") // want "log.Fatal is forbidden outside main.main"
	process.Exit(1)        // want "os.Exit is forbidden outside main.main"
}

func calls() {
	logger.Fatalf("%s", "failed") // want "log.Fatalf is forbidden outside main.main"
	logger.Fatalln("failed")      // want "log.Fatalln is forbidden outside main.main"
	(process.Exit)(1)             // want "os.Exit is forbidden outside main.main"
	(panic)("failed")             // want "builtin panic is forbidden"
	defer process.Exit(1)         // want "os.Exit is forbidden outside main.main"
	go logger.Fatal("failed")     // want "log.Fatal is forbidden outside main.main"
	logger.Print("allowed")
}

var callback = func() {
	process.Exit(1) // want "os.Exit is forbidden outside main.main"
}
