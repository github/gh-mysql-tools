package core

import (
	"encoding/json"
	"testing"

	test "github.com/openark/golib/tests"
)

func TestRepositoryUnmarshall(t *testing.T) {
	{
		j := `
			{
		    "id": 8,
		    "org": "myorg",
		    "repo": "skeefree-test",
		    "owner": "database-team",
		    "added_timestamp": "2019-05-23T00:43:27Z",
		    "updated_timestamp": "2019-05-23T00:43:27Z"
		  }
		`
		r := &Repository{}
		_ = json.Unmarshal([]byte(j), &r)
		test.S(t).ExpectEquals(r.Id, int64(8))
		test.S(t).ExpectEquals(r.Org, "myorg")
		test.S(t).ExpectEquals(r.Repo, "skeefree-test")
		test.S(t).ExpectEquals(r.Owner, "database-team")
		test.S(t).ExpectEquals(r.MySQLCluster, "")
		test.S(t).ExpectEquals(r.MySQLSchema, "")

		test.S(t).ExpectEquals(r.OrgRepo(), "myorg/skeefree-test")
	}
}
