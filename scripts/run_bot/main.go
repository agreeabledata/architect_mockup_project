// Command run_bot prints a scripted decision-bot run against the real engine.
// No LLM, no window: Observe → StepBot until the ledger closes.
package main

import (
	"flag"
	"fmt"
	"os"

	"architect_mockup_project/engine"
)

func main() {
	seed := flag.Uint("seed", 1, "world seed")
	flag.Parse()

	w := engine.New(uint32(*seed))
	for n := 0; n < 256; n++ {
		st := w.Observe()
		fmt.Printf("======== step %d ========\n%sbalance=%d days=%d done=%v\n",
			n, st.Narrative, st.Balance, st.HealthDays, st.Done)
		if st.Done {
			return
		}
		out, err := w.StepBot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "bot: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("earned=%d\n%s\n\n", out.Earned, out.Narrative)
	}
	fmt.Fprintln(os.Stderr, "bot: step cap hit")
	os.Exit(1)
}
