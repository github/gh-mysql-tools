package core

import (
	"testing"

	test "github.com/openark/golib/tests"
)

func TestHasStatus(t *testing.T) {
	{
		pr := NewPullRequest()
		test.S(t).ExpectEquals(pr.GetStatus(), PullRequestStatusDetected)
	}
	{
		pr := NewPullRequest()
		pr.Status = "complete"
		test.S(t).ExpectEquals(pr.GetStatus(), PullRequestStatusComplete)
	}
	{
		pr := NewPullRequest()
		pr.Status = ""
		test.S(t).ExpectEquals(pr.GetStatus(), PullRequestStatusUnknown)
	}
}
