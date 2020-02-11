package util

import (
	"testing"

	test "github.com/openark/golib/tests"
)

func TestParseOrgRepo(t *testing.T) {
	{
		params := map[string]string{
			"repo": "myorg/myrepo",
		}
		org, repo, err := ParseOrgRepo(params, "def")
		test.S(t).ExpectNil(err)
		test.S(t).ExpectEquals(org, "myorg")
		test.S(t).ExpectEquals(repo, "myrepo")
	}
	{
		params := map[string]string{
			"repo": "myrepo",
		}
		org, repo, err := ParseOrgRepo(params, "def")
		test.S(t).ExpectNil(err)
		test.S(t).ExpectEquals(org, "def")
		test.S(t).ExpectEquals(repo, "myrepo")
	}
	{
		params := map[string]string{
			"org":  "myorg",
			"repo": "myrepo",
		}
		org, repo, err := ParseOrgRepo(params, "def")
		test.S(t).ExpectNil(err)
		test.S(t).ExpectEquals(org, "myorg")
		test.S(t).ExpectEquals(repo, "myrepo")
	}
	{
		params := map[string]string{
			"org":  "myorg",
			"repo": "explicit-org/myrepo",
		}
		org, repo, err := ParseOrgRepo(params, "def")
		test.S(t).ExpectNil(err)
		test.S(t).ExpectEquals(org, "explicit-org")
		test.S(t).ExpectEquals(repo, "myrepo")
	}
	{
		params := map[string]string{
			"repo": "myorg/myrepo/mypath",
		}
		_, _, err := ParseOrgRepo(params, "def")
		test.S(t).ExpectNotNil(err)
	}
}
