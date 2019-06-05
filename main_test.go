package main

import (
	"flag"
	"os"
	"testing"
)

//integration test flag
var (
	integration = flag.Bool("integration", false, "run integration tests")
)

func TestStartServer(t *testing.T) {
	flag.Parse()
	if !*integration {
		t.Skip("skipping TestStartServer")
	}

	os.Setenv("R_CMD", "/usr/local/bin/R")

	startServer()
}
