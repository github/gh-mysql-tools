package ghapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/github/skeefree/go/config"
	"github.com/google/go-github/github"
	"github.com/patrickmn/go-cache"
)

const (
	MigrationDiffLabel                  string = "migration:skeema:diff"
	MigrationDetectedLabel              string = "migration:skeefree:detected"
	MigrationQueuedLabel                string = "migration:skeefree:queued"
	MigrationApprovedByDBReviewersLabel string = "migration:approved:schema-reviewers"
	MigrationApprovedByDBInfraLabel     string = "migration:approved:database-team"
	MigrationForReviewLabel             string = "migration:for:review"
	MigrationForReviewAlternateLabel    string = "DB migration" // Used internally at GitHub for backwards compatability

	CheckRunSuccessfulConclusion string = "success"
)
const magicCommentHint = "<!-- skeema:magic:comment -->"

// teamMembers cache: key=team-slug; value = map[string]bool
var teamMembers = cache.New(time.Hour, 10*time.Minute)
var listOptions = github.ListOptions{PerPage: 100}

type GitHubAPI struct {
	client *github.Client
}

func NewGitHubAPI(c *config.Config) (*GitHubAPI, error) {
	client, err := newGitHubClient(c)
	if err != nil {
		return nil, err
	}

	return &GitHubAPI{client: client}, nil
}

// ValidateRepo validates that a requested repository exists and is accessible
func (c *GitHubAPI) ValidateRepo(ctx context.Context, org, repo string) (repository *github.Repository, err error) {
	repository, _, err = c.client.Repositories.Get(ctx, org, repo)
	return repository, err
}

// IsTeamMember checks if a given user (login) is member of given team (slug)
// The result of this function is cached.
func (c *GitHubAPI) IsTeamMember(ctx context.Context, org, user string, teamSlug string) (isMember bool, err error) {
	if item, found := teamMembers.Get(teamSlug); found {
		members := item.(map[string]bool)
		return members[user], nil
	}

	team, _, err := c.client.Teams.GetTeamBySlug(ctx, org, teamSlug)
	if err != nil {
		return false, err
	}
	membersMap := make(map[string]bool)
	members, _, err := c.client.Teams.ListTeamMembers(ctx, team.GetID(), nil)
	if err != nil {
		return false, err
	}
	for _, u := range members {
		membersMap[u.GetLogin()] = true
	}
	teamMembers.SetDefault(teamSlug, membersMap)
	return membersMap[user], nil
}

// ValidateTeam validates a team owns a given repo
func (c *GitHubAPI) GetAdminTeams(ctx context.Context, org, repo string) (adminTeams []*github.Team, err error) {
	teams, _, err := c.client.Repositories.ListTeams(ctx, org, repo, nil)
	if err != nil {
		return adminTeams, err
	}
	for _, team := range teams {
		if *team.Permission == "admin" {
			adminTeams = append(adminTeams, team)
		}
	}
	return adminTeams, err
}

// ValidateTeam validates a team owns a given repo
func (c *GitHubAPI) ValidateAdminTeam(ctx context.Context, org, repo string, teamSlug string) (isAdmin bool, err error) {
	team, _, err := c.client.Teams.GetTeamBySlug(ctx, org, teamSlug)
	if err != nil {
		return false, err
	}
	r, _, err := c.client.Teams.IsTeamRepo(ctx, team.GetID(), org, repo)
	if err != nil {
		return false, err
	}
	if r == nil {
		return false, fmt.Errorf("team validation error: team %s is not an owner of %s/%s", teamSlug, org, repo)
	}
	return true, nil
}

// ValidateAdminUser validates that a user is an admin of a repo
func (c *GitHubAPI) ValidateAdminUser(ctx context.Context, org, repo string, user string) (isAdmin bool, err error) {
	permission, _, err := c.client.Repositories.GetPermissionLevel(ctx, org, repo, user)
	if err != nil {
		return false, err
	}
	return permission.GetPermission() == "admin", nil
}

// ReadPullRequest reads and returns a pull request
func (c *GitHubAPI) ReadPullRequest(ctx context.Context, org, repo string, number int) (pr *github.PullRequest, err error) {
	pr, _, err = c.client.PullRequests.Get(ctx, org, repo, number)
	return pr, err
}

// ReadPullRequest reads and returns a pull request
func (c *GitHubAPI) ReadPullRequestApprovedByAdmin(ctx context.Context, org, repo string, number int) (user *github.User, err error) {
	reviews, _, err := c.client.PullRequests.ListReviews(ctx, org, repo, number, nil)
	if err != nil {
		return nil, err
	}
	for _, review := range reviews {
		if review.GetState() != "APPROVED" {
			// We're only interested in approved PRs.
			continue
		}
		isAdmin, err := c.ValidateAdminUser(ctx, org, repo, review.GetUser().GetLogin())
		if err != nil {
			return nil, err
		}
		if isAdmin {
			return review.GetUser(), nil
		}
	}
	return nil, fmt.Errorf("Not approved by admin")
}

// ReadPullRequest reads and returns a pull request
func (c *GitHubAPI) PullRequestApprovedByTeam(ctx context.Context, org, repo string, number int, teamSlug string) (approvedByTeam bool, err error) {
	reviews, _, err := c.client.PullRequests.ListReviews(ctx, org, repo, number, nil)
	if err != nil {
		return false, err
	}
	for _, review := range reviews {
		if review.GetState() != "APPROVED" {
			// We're only interested in approved PRs.
			continue
		}
		isMember, err := c.IsTeamMember(ctx, org, review.GetUser().GetLogin(), teamSlug)
		if err != nil {
			return false, err
		}
		if isMember {
			return true, nil
		}
	}
	return false, nil
}

// PullRequestApprovedBySomeone sees if there's at least one APPROVED review on the PR.
func (c *GitHubAPI) PullRequestApprovedBySomeone(ctx context.Context, org, repo string, number int) (approved bool, err error) {
	reviews, _, err := c.client.PullRequests.ListReviews(ctx, org, repo, number, nil)
	if err != nil {
		return false, err
	}
	for _, review := range reviews {
		if review.GetState() == "APPROVED" {
			return true, nil
		}
	}
	return false, nil
}

// ReadPullRequest reads and returns a pull request
func (c *GitHubAPI) ReadPullRequestSkeemaDiffMagicComment(ctx context.Context, org, repo string, number int) (ic *github.IssueComment, err error) {
	comments, _, err := c.client.Issues.ListComments(ctx, org, repo, number, &github.IssueListCommentsOptions{ListOptions: listOptions})
	if err != nil {
		return ic, err
	}
	for _, comment := range comments {
		if strings.HasPrefix(comment.GetBody(), magicCommentHint) {
			return comment, nil
		}
	}
	return nil, fmt.Errorf("ReadPullRequestSkeemaDiffMagicComment: could not find magic comment")
}

// AddPullRequestComment adds a comment to a PR
func (c *GitHubAPI) AddPullRequestComment(ctx context.Context, org, repo string, number int, comment string) (ic *github.IssueComment, err error) {
	if _, err := c.ReadPullRequest(ctx, org, repo, number); err != nil {
		return nil, err
	}
	ic = &github.IssueComment{
		Body: &comment,
	}
	ic, _, err = c.client.Issues.CreateComment(ctx, org, repo, number, ic)
	return ic, err
}

// AddPullRequestLabel adds a label to a PR. The label should exist beforehand.
func (c *GitHubAPI) RemovePullRequestLabel(ctx context.Context, org, repo string, number int, label string) (err error) {
	_, err = c.client.Issues.RemoveLabelForIssue(ctx, org, repo, number, label)
	return err
}

// AddPullRequestLabel adds a label to a PR. The label should exist beforehand.
func (c *GitHubAPI) AddPullRequestLabel(ctx context.Context, org, repo string, number int, label string) (err error) {
	_, _, err = c.client.Issues.AddLabelsToIssue(ctx, org, repo, number, []string{label})
	return err
}

// RequestPullRequestReview requests a review on a given PR from a given team
func (c *GitHubAPI) RequestPullRequestReview(ctx context.Context, org, repo string, number int, teamSlug string) (newlyRequested bool, err error) {
	alreadyRequested, err := c.TeamRequestedToReviewPullRequest(ctx, org, repo, number, teamSlug)
	if err != nil {
		return false, err
	}
	if alreadyRequested {
		// no need to re-request
		return false, nil
	}
	reviewers := github.ReviewersRequest{
		TeamReviewers: []string{teamSlug},
	}
	_, _, err = c.client.PullRequests.RequestReviewers(ctx, org, repo, number, reviewers)
	return true, err
}

// TeamRequestedToReviewPullRequest checks if a team is already requested to review a PR
func (c *GitHubAPI) TeamRequestedToReviewPullRequest(ctx context.Context, org, repo string, number int, teamSlug string) (requested bool, err error) {
	reviewers, _, err := c.client.PullRequests.ListReviewers(ctx, org, repo, number, nil)
	if err != nil {
		return false, err
	}
	for _, team := range reviewers.Teams {
		if team.GetSlug() == teamSlug {
			return true, nil
		}
	}
	return false, nil
}

func (c *GitHubAPI) SearchSkeemaDiffUndetectedPRs(ctx context.Context, orgRepo string) (issuesSearchResult *github.IssuesSearchResult, searchString string, err error) {
	searchString = fmt.Sprintf("repo:%s is:pr state:open label:%s -label:%s", orgRepo, MigrationDiffLabel, MigrationDetectedLabel)
	issuesSearchResult, _, err = c.client.Search.Issues(ctx, searchString, &github.SearchOptions{ListOptions: listOptions})
	return issuesSearchResult, searchString, err
}

func (c *GitHubAPI) IsSkeemaCheckStatusPassingForPullRequest(ctx context.Context, org, repo string, number int, skeemaCheckName string) (passing bool, err error) {
	pr, err := c.ReadPullRequest(ctx, org, repo, number)
	if err != nil {
		return false, err
	}

	headRef := pr.GetHead().GetRef()

	opts := &github.ListCheckRunsOptions{
		CheckName: &skeemaCheckName,
	}

	results, _, err := c.client.Checks.ListCheckRunsForRef(ctx, org, repo, headRef, opts)
	if err != nil {
		return false, err
	}

	if len(results.CheckRuns) == 0 {
		return false, fmt.Errorf("Could not find skeema check with name \"%s\" for org %s repo %s pull request %d", skeemaCheckName, org, repo, number)
	}

	return *results.CheckRuns[0].Conclusion == CheckRunSuccessfulConclusion, nil
}
