// Command gh-ff-merge is a gh extension that merges a pull request by
// fast-forwarding its base branch to the pull request head.
package main

import (
	"fmt"
	"os"

	"github.com/Dekker1/gh-ff-merge/internal/ffmerge"
	"github.com/spf13/pflag"
)

const usage = `Merge a pull request by fast-forwarding its base branch.

Usage:
  gh ff-merge [flags] [<number> | <url> | <branch>]

With no argument, the pull request for the current branch is used. A branch
name only works inside a local checkout.

Flags:
  -R, --repo OWNER/REPO   Select another repository
  -n, --dry-run           Run the checks, leave the base branch alone
  -d, --delete-branch     Delete the head branch after merging
  -h, --help              Show this help text
`

func main() {
	var opts ffmerge.Options

	flags := pflag.NewFlagSet("gh ff-merge", pflag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flags.StringVarP(&opts.Repo, "repo", "R", "", "Select another repository")
	flags.BoolVarP(&opts.DryRun, "dry-run", "n", false, "Run the checks, leave the base branch alone")
	flags.BoolVarP(&opts.DeleteBranch, "delete-branch", "d", false, "Delete the head branch after merging")
	help := flags.BoolP("help", "h", false, "Show this help text")

	if err := flags.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		flags.Usage()
		os.Exit(1)
	}
	if *help {
		fmt.Print(usage)
		return
	}
	if flags.NArg() > 1 {
		fmt.Fprintf(os.Stderr, "accepts at most one pull request argument, got %d\n", flags.NArg())
		os.Exit(1)
	}
	opts.Arg = flags.Arg(0)

	client, err := ffmerge.NewClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := ffmerge.Run(os.Stdout, client, opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(ffmerge.ExitCode(err))
	}
}
