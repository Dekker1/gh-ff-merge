package ffmerge

import (
	"errors"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

// fakeClient records what Run asks of the API and replays canned answers.
type fakeClient struct {
	pr     PR
	status string

	viewErr   error
	cmpErr    error
	updateErr error
	deleteErr error

	updated []string
	deleted []string
}

func (f *fakeClient) ViewPR(repo, arg string) (PR, error) { return f.pr, f.viewErr }

func (f *fakeClient) Compare(repo, base, sha string) (string, error) {
	return f.status, f.cmpErr
}

func (f *fakeClient) UpdateRef(repo, branch, sha string) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = append(f.updated, repo+" "+branch+" "+sha)
	return nil
}

func (f *fakeClient) DeleteRef(repo, branch string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, repo+" "+branch)
	return nil
}

func openPR() PR {
	return PR{
		BaseRefName: "main",
		HeadRefName: "feature",
		HeadRefOid:  "0123456789abcdef0123456789abcdef01234567",
		State:       "OPEN",
		URL:         "https://github.com/octo/repo/pull/7",
	}
}

func run(t *testing.T, c *fakeClient, opts Options) (string, error) {
	t.Helper()
	var out strings.Builder
	err := Run(&out, c, opts)
	return out.String(), err
}

func TestAheadFastForwards(t *testing.T) {
	c := &fakeClient{pr: openPR(), status: "ahead"}
	out, err := run(t, c, Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := "octo/repo main 0123456789abcdef0123456789abcdef01234567"
	if len(c.updated) != 1 || c.updated[0] != want {
		t.Errorf("updated = %v, want [%s]", c.updated, want)
	}
	if !strings.Contains(out, "fast-forwarded main to 0123456") {
		t.Errorf("output = %q", out)
	}
}

func TestIdenticalDoesNothing(t *testing.T) {
	c := &fakeClient{pr: openPR(), status: "identical"}
	out, err := run(t, c, Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(c.updated) != 0 {
		t.Errorf("updated = %v, want none", c.updated)
	}
	if !strings.Contains(out, "nothing to do") {
		t.Errorf("output = %q", out)
	}
}

func TestNotFastForwardable(t *testing.T) {
	for _, status := range []string{"behind", "diverged", "surprise"} {
		t.Run(status, func(t *testing.T) {
			c := &fakeClient{pr: openPR(), status: status}
			_, err := run(t, c, Options{})
			if err == nil {
				t.Fatal("Run: want an error")
			}
			if got := ExitCode(err); got != 1 {
				t.Errorf("ExitCode = %d, want 1", got)
			}
			if len(c.updated) != 0 {
				t.Errorf("updated = %v, want none", c.updated)
			}
		})
	}
}

func TestRefusesClosedAndDraft(t *testing.T) {
	tests := map[string]struct {
		pr   func(PR) PR
		want string
	}{
		"closed": {func(pr PR) PR { pr.State = "CLOSED"; return pr }, "not open"},
		"merged": {func(pr PR) PR { pr.State = "MERGED"; return pr }, "not open"},
		"draft":  {func(pr PR) PR { pr.IsDraft = true; return pr }, "draft"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &fakeClient{pr: tc.pr(openPR()), status: "ahead"}
			_, err := run(t, c, Options{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
			if got := ExitCode(err); got != 1 {
				t.Errorf("ExitCode = %d, want 1", got)
			}
			if len(c.updated) != 0 {
				t.Errorf("updated = %v, want none", c.updated)
			}
		})
	}
}

func TestAPIRejectionKeepsExitCodeAndMessage(t *testing.T) {
	rejection := &Error{Code: 2, Msg: "At least 1 approving review is required"}
	c := &fakeClient{pr: openPR(), status: "ahead", updateErr: rejection}
	_, err := run(t, c, Options{})
	if !errors.Is(err, rejection) {
		t.Fatalf("err = %v, want the API rejection", err)
	}
	if got := ExitCode(err); got != 2 {
		t.Errorf("ExitCode = %d, want 2", got)
	}
}

func TestDryRunSkipsTheRefUpdate(t *testing.T) {
	c := &fakeClient{pr: openPR(), status: "ahead"}
	out, err := run(t, c, Options{DryRun: true, DeleteBranch: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(c.updated) != 0 || len(c.deleted) != 0 {
		t.Errorf("updated = %v, deleted = %v, want none", c.updated, c.deleted)
	}
	if !strings.Contains(out, "dry run") {
		t.Errorf("output = %q", out)
	}
}

func TestDeleteBranchSameRepo(t *testing.T) {
	c := &fakeClient{pr: openPR(), status: "ahead"}
	out, err := run(t, c, Options{DeleteBranch: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(c.deleted) != 1 || c.deleted[0] != "octo/repo feature" {
		t.Errorf("deleted = %v, want [octo/repo feature]", c.deleted)
	}
	if !strings.Contains(out, "deleted branch feature") {
		t.Errorf("output = %q", out)
	}
}

func TestDeleteBranchKeepsForkBranch(t *testing.T) {
	pr := openPR()
	pr.IsCrossRepository = true
	c := &fakeClient{pr: pr, status: "ahead"}
	out, err := run(t, c, Options{DeleteBranch: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(c.updated) != 1 {
		t.Errorf("updated = %v, want the fast-forward", c.updated)
	}
	if len(c.deleted) != 0 {
		t.Errorf("deleted = %v, want none", c.deleted)
	}
	if !strings.Contains(out, "another repository") {
		t.Errorf("output = %q", out)
	}
}

func TestViewErrorStops(t *testing.T) {
	c := &fakeClient{viewErr: &Error{Code: 1, Msg: "no pull requests found"}}
	if _, err := run(t, c, Options{}); err == nil {
		t.Fatal("Run: want an error")
	}
}

func TestUpdateRefErrorHintsWorkflowScope(t *testing.T) {
	err := updateRefError(&api.HTTPError{StatusCode: 404, Message: "Not Found"})
	if !strings.Contains(err.Error(), "Not Found") || !strings.Contains(err.Error(), "gh auth refresh -s workflow") {
		t.Errorf("err = %q, want the API message and the workflow hint", err)
	}
	if got := ExitCode(err); got != 2 {
		t.Errorf("ExitCode = %d, want 2", got)
	}
	err = updateRefError(&api.HTTPError{StatusCode: 422, Message: "Update is not a fast forward"})
	if strings.Contains(err.Error(), "workflow") {
		t.Errorf("err = %q, want no workflow hint", err)
	}
}

func TestRepoFromURL(t *testing.T) {
	tests := map[string]string{
		"https://github.com/octo/repo/pull/7":       "octo/repo",
		"https://ghe.example.com/team/tool/pull/12": "team/tool",
		"nonsense": "",
	}
	for in, want := range tests {
		got, err := repoFromURL(in)
		if want == "" {
			if err == nil {
				t.Errorf("repoFromURL(%q) = %q, want an error", in, got)
			}
			continue
		}
		if err != nil || got != want {
			t.Errorf("repoFromURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}
