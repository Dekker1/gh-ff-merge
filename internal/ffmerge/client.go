package ffmerge

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/api"
)

// prFields are the "gh pr view" JSON fields Run reads.
const prFields = "baseRefName,headRefOid,headRefName,state,isDraft,isCrossRepository,url"

type ghClient struct {
	rest *api.RESTClient
}

// NewClient returns a Client backed by the gh CLI and the GitHub REST API.
func NewClient() (Client, error) {
	rest, err := api.DefaultRESTClient()
	if err != nil {
		return nil, err
	}
	return &ghClient{rest: rest}, nil
}

func (g *ghClient) ViewPR(repo, arg string) (PR, error) {
	args := []string{"pr", "view"}
	if arg != "" {
		args = append(args, arg)
	}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	args = append(args, "--json", prFields)

	stdout, stderr, err := gh.Exec(args...)
	if err != nil {
		// gh already explains itself, so pass its message on unchanged.
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return PR{}, &Error{Code: 1, Msg: msg}
		}
		return PR{}, err
	}
	var pr PR
	if err := json.Unmarshal(stdout.Bytes(), &pr); err != nil {
		return PR{}, fmt.Errorf("cannot read the pull request details: %w", err)
	}
	return pr, nil
}

func (g *ghClient) Compare(repo, base, sha string) (string, error) {
	var resp struct{ Status string }
	path := fmt.Sprintf("repos/%s/compare/%s...%s", repo, base, sha)
	if err := g.rest.Get(path, &resp); err != nil {
		return "", apiError(err)
	}
	return resp.Status, nil
}

func (g *ghClient) UpdateRef(repo, branch, sha string) error {
	path := fmt.Sprintf("repos/%s/git/refs/heads/%s", repo, branch)
	body := strings.NewReader(fmt.Sprintf(`{"sha":%q,"force":false}`, sha))
	if err := g.rest.Patch(path, body, nil); err != nil {
		return apiError(err)
	}
	return nil
}

func (g *ghClient) DeleteRef(repo, branch string) error {
	path := fmt.Sprintf("repos/%s/git/refs/heads/%s", repo, branch)
	if err := g.rest.Delete(path, nil); err != nil {
		return apiError(err)
	}
	return nil
}

// apiError reports a rejection by the API using GitHub's own wording, because
// GitHub owns the merge rules, not this extension.
func apiError(err error) error {
	var httpErr *api.HTTPError
	if !errors.As(err, &httpErr) {
		return err
	}
	msg := []string{httpErr.Message}
	for _, item := range httpErr.Errors {
		if item.Message != "" {
			msg = append(msg, item.Message)
		}
	}
	if httpErr.Message == "" {
		msg[0] = httpErr.Error()
	}
	return &Error{Code: 2, Msg: strings.Join(msg, "\n")}
}
