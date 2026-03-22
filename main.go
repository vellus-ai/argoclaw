package main

import (
	_ "time/tzdata" // embed IANA timezone database for containers without tzdata

	"github.com/vellus-ai/argoclaw/cmd"
)

func main() {
	cmd.Execute()
}
