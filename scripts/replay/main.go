// Command replay prints a run's decisions in order from its transcript.
//
//	go run ./scripts/replay transcripts/run_….jsonl
package main

import (
	"fmt"
	"os"

	"architect_mockup_project/harness"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Fprintln(os.Stderr, "usage: replay <transcript.jsonl>")
		os.Exit(2)
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		fail("%v", err)
	}
	defer f.Close()

	lines, err := harness.Replay(f)
	for _, line := range lines {
		fmt.Println(line)
	}
	if err != nil {
		fail("replay: %v", err)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
