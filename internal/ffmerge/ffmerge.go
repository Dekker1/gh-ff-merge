// Package ffmerge merges a GitHub pull request by fast-forwarding its base
// branch to the pull request head, using only the GitHub API.
package ffmerge

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// PR holds the pull request fields ff-merge needs, as "gh pr view" reports them.
type PR struct {
	BaseRefName       string
	HeadRefName       string
	HeadRefOid        string
	IsCrossRepository bool
	IsDraft           bool
	State             string
	URL               string
}

// Client is the set of remote operations ff-merge performs.
type Client interface {
	// ViewPR resolves a pull request the way "gh pr view" does. arg is a
	// number, a URL, a branch name, or empty for the current branch. repo is
	// the value of -R, empty when the flag was not given.
	ViewPR(repo, arg string) (PR, error)
	// Compare returns the status field of a base...head comparison.
	Compare(repo, base, sha string) (string, error)
	// UpdateRef moves branch to sha without force, so the server rejects
	// anything that is not a fast-forward.
	UpdateRef(repo, branch, sha string) error
	// DeleteRef deletes branch.
	DeleteRef(repo, branch string) error
}

// Options configures a single Run.
type Options struct {
	// Repo is the -R value, empty to let gh resolve the repository.
	Repo string
	// Arg is a pull request number, URL or branch name. Empty means the
	// pull request for the current branch.
	Arg          string
	DryRun       bool
	DeleteBranch bool
}

// Error is a failure that carries the exit code the process should use.
type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

// ExitCode returns the exit code to use for err.
func ExitCode(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return 1
}

// precondition reports a local check that failed before any ref was touched.
func precondition(format string, a ...any) *Error {
	return &Error{Code: 1, Msg: fmt.Sprintf(format, a...)}
}

// Run fast-forwards the base branch of the pull request named by opts.
func Run(out io.Writer, c Client, opts Options) error {
	pr, err := c.ViewPR(opts.Repo, opts.Arg)
	if err != nil {
		return err
	}
	if pr.State != "OPEN" {
		return precondition("pull request is %s, not open", strings.ToLower(pr.State))
	}
	if pr.IsDraft {
		return precondition("pull request is a draft")
	}
	repo, err := repoFromURL(pr.URL)
	if err != nil {
		return err
	}

	status, err := c.Compare(repo, pr.BaseRefName, pr.HeadRefOid)
	if err != nil {
		return err
	}
	switch status {
	case "ahead":
	case "identical":
		fmt.Fprintf(out, "%s already points at %s, nothing to do\n", pr.BaseRefName, short(pr.HeadRefOid))
		return nil
	case "behind":
		return precondition("the pull request is behind %s; rebase it first", pr.BaseRefName)
	case "diverged":
		return precondition("the pull request has diverged from %s; rebase it first", pr.BaseRefName)
	default:
		return precondition("unexpected comparison status %q for %s...%s", status, pr.BaseRefName, short(pr.HeadRefOid))
	}

	if opts.DryRun {
		fmt.Fprintf(out, "dry run: %s can be fast-forwarded to %s\n%s\n", pr.BaseRefName, short(pr.HeadRefOid), pr.URL)
		return nil
	}
	if err := c.UpdateRef(repo, pr.BaseRefName, pr.HeadRefOid); err != nil {
		return err
	}
	fmt.Fprintf(out, "fast-forwarded %s to %s\n%s\n", pr.BaseRefName, short(pr.HeadRefOid), pr.URL)

	if opts.DeleteBranch {
		if pr.IsCrossRepository {
			fmt.Fprintf(out, "head branch %s lives in another repository, not deleting it\n", pr.HeadRefName)
			return nil
		}
		if err := c.DeleteRef(repo, pr.HeadRefName); err != nil {
			return err
		}
		fmt.Fprintf(out, "deleted branch %s\n", pr.HeadRefName)
	}
	return nil
}

// repoFromURL reads the OWNER/REPO of the base repository out of a pull
// request URL, so the API calls work outside a local checkout too.
func repoFromURL(prURL string) (string, error) {
	if u, err := url.Parse(prURL); err == nil {
		parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			return parts[0] + "/" + parts[1], nil
		}
	}
	return "", precondition("cannot read the repository from the pull request URL %q", prURL)
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
