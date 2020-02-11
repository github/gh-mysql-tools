package app

import (
	"testing"

	test "github.com/openark/golib/tests"
)

func TestGetActionName(t *testing.T) {
	{
		test.S(t).ExpectEquals(getActionName("my-default-repo"), defaultActionName)
	}
	{
		test.S(t).ExpectEquals(getActionName("myorg"), "skeema-diff")
	}
}
