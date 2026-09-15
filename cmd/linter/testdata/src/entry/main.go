package main

import (
	"log"
	"os"
)

func main() {
	log.Fatal("allowed")
	log.Fatalf("%s", "allowed")
	log.Fatalln("allowed")
	os.Exit(0)
	panic("forbidden even in main") // want "builtin panic is forbidden"
	func() {
		os.Exit(1)          // want "os.Exit is forbidden outside main.main"
		log.Fatal("nested") // want "log.Fatal is forbidden outside main.main"
	}()
	os.Exit(0)
}

func helper() {
	os.Exit(1)          // want "os.Exit is forbidden outside main.main"
	log.Fatal("failed") // want "log.Fatal is forbidden outside main.main"
	panic("failed")     // want "builtin panic is forbidden"
}

func init() {
	os.Exit(1) // want "os.Exit is forbidden outside main.main"
}

type application struct{}

func (application) main() {
	os.Exit(1) // want "os.Exit is forbidden outside main.main"
}
