package util

import (
	"fmt"
	"regexp"
)

var (
	orgRepoParserRegexp     = regexp.MustCompile("^([^/]+)/([^/]+)$")
	orglessRepoParserRegexp = regexp.MustCompile("^([^/]+)$")
)

func ParseOrgRepo(params map[string]string, defaultOrg string) (org string, repo string, err error) {

	if submatch := orgRepoParserRegexp.FindStringSubmatch(params["repo"]); len(submatch) > 0 {
		// <org> explcitily indicated in "repo" params as in "github/freno"
		org, repo = submatch[1], submatch[2]
	} else if submatch := orglessRepoParserRegexp.FindStringSubmatch(params["repo"]); len(submatch) > 0 {
		if params["org"] != "" {
			org = params["org"]
		} else {
			org = defaultOrg
		}
		repo = submatch[1]
	} else {
		err = fmt.Errorf("Unable to parse org/repo from %+v", params)
	}
	return org, repo, err
}
