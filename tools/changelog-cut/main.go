// Command changelog-cut folds the pending docs/changelog/ entries into a new
// version section of CHANGELOG.md. Run it through `make changelog-cut
// RELEASE=X.Y.Z` in a release PR; see docs/changelog/README.md.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/qase-tms/tiden-mcp-server/internal/changelogcut"
)

func main() {
	version := flag.String("version", "", "release version, X.Y.Z (required)")
	date := flag.String("date", time.Now().Format(time.DateOnly), "release date, YYYY-MM-DD")
	flag.Parse()
	if *version == "" {
		fmt.Fprintln(os.Stderr, "usage: changelog-cut -version X.Y.Z [-date YYYY-MM-DD]")
		os.Exit(2)
	}

	summary, err := changelogcut.Run(".", *version, *date)
	if err != nil {
		fmt.Fprintln(os.Stderr, "changelog-cut:", err)
		os.Exit(1)
	}
	fmt.Print(summary)
}
