package app

import (
	"strings"
)

const defaultActionName = "skeema-diff"

var specialReposActionName = map[string]string{}

var reposRequiringDBReviewers = map[string]bool{
	"special-repo": true,
}

func init() {
}

var postCompletePRComments = map[string]string{
	"special-repo": strings.ReplaceAll(`
#### For this special-repo, you are kindly asked to follow this flow:

markdown §supported§, with this §funny§ character as backtick replacement.
`, "§", "`"),
}

func getActionName(repo string) string {
	if name, ok := specialReposActionName[repo]; ok {
		return name
	}
	return defaultActionName
}
