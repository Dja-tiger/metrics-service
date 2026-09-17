package dotimports

import (
	. "log"
	. "os"
)

func calls() {
	Fatal("failed") // want "log.Fatal is forbidden outside main.main"
	Exit(1)         // want "os.Exit is forbidden outside main.main"
}
