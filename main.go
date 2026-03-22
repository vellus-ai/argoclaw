package main

import (
	_ "time/tzdata" // embed IANA timezone database for containers without tzdata

	"github.com/vellus-ai/arargoclaw/cmd"
)

func main() {
	cmd.Execute()
}
