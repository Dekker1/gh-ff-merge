# gh ff-merge

GitHub offers no fast-forward merge: every button either adds a merge commit or,
under "Rebase and merge", rewrites your commits under new SHAs. This `gh`
extension merges a pull request by moving the base branch to the pull request
head instead, so the commits you reviewed are the commits on the base branch.

Because nothing is rewritten, commit signatures survive the merge. The
extension talks to the GitHub API only: no clone, no fetch, no local git.

## Install

```sh
gh extension install Dekker1/gh-ff-merge
```

Optionally, alias it next to the built-in merge command (needs a recent gh, which
allows aliases for subcommands):

```sh
gh alias set 'pr ff-merge' 'ff-merge'
```

## Usage

```
gh ff-merge [flags] [<number> | <url> | <branch>]
```

| Flag | Meaning |
| --- | --- |
| `-R`, `--repo OWNER/REPO` | Select another repository |
| `-n`, `--dry-run` | Run the checks, leave the base branch alone |
| `-d`, `--delete-branch` | Delete the head branch after merging |
| `-h`, `--help` | Show the help text |

With no argument, the pull request for the current branch is used, exactly as
`gh pr view` resolves it. A branch name only works inside a local checkout.

```sh
# The pull request for the branch you are on
gh ff-merge

# By number, and drop the head branch afterwards
gh ff-merge 42 --delete-branch

# By URL, from anywhere
gh ff-merge https://github.com/cli/cli/pull/1

# Check a pull request in another repository without touching it
gh ff-merge -R cli/cli 1 --dry-run
```

The command refuses to run unless the pull request is open, is not a draft, and
is strictly ahead of its base branch. A pull request that is behind or has
diverged has to be rebased first. On success it prints the base branch, the new
SHA and the pull request URL; GitHub marks the pull request as merged by itself.

Exit codes: `0` merged or nothing to do, `1` a precondition failed, `2` the API
rejected the update.

## Branch protection

The ref update goes through the normal API, so all your branch protection rules
still apply. The pull request has to satisfy required reviews and required
status checks, and the extension reports GitHub's own message when it does not.
Nothing is enforced client side.

Merge queues are not supported: a queue takes over the merge itself, so use the
queue instead of this extension on those branches.

## License

MIT
