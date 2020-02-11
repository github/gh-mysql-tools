package core

import (
	"fmt"
	"time"
)

type PullRequestStatus string

const (
	PullRequestStatusDetected  PullRequestStatus = "detected"
	PullRequestStatusQueued    PullRequestStatus = "queued"
	PullRequestStatusCancelled PullRequestStatus = "cancelled"
	PullRequestStatusComplete  PullRequestStatus = "complete"
	PullRequestStatusUnknown   PullRequestStatus = "unknown"
)

type PullRequestPriority int

const (
	PullRequestPriorityUrgent PullRequestPriority = 2
	PullRequestPriorityHigh   PullRequestPriority = 1
	PullRequestPriorityNormal PullRequestPriority = 0
	PullRequestPriorityLow    PullRequestPriority = -1
)

func (p PullRequestPriority) ToText() string {
	switch p {
	case PullRequestPriorityUrgent:
		return "urgent"
	case PullRequestPriorityHigh:
		return "high"
	case PullRequestPriorityNormal:
		return "normal"
	case PullRequestPriorityLow:
		return "low"
	}
	return "unknown"
}

func PullRequestPriorityFromText(p string) PullRequestPriority {
	switch p {
	case "urgent":
		return PullRequestPriorityUrgent
	case "high":
		return PullRequestPriorityHigh
	case "normal":
		return PullRequestPriorityNormal
	case "low":
		return PullRequestPriorityLow
	}
	return PullRequestPriorityNormal
}

type PullRequest struct {
	Id                           int64               `db:"id" json:"id"`
	Org                          string              `db:"org" json:"org"`
	Repo                         string              `db:"repo" json:"repo"`
	Number                       int                 `db:"pull_request_number" json:"pull_request_number"`
	Title                        string              `db:"title" json:"title"`
	Author                       string              `db:"author" json:"author"`
	Priority                     PullRequestPriority `db:"priority" json:"priority"`
	Status                       string              `db:"status" json:"status"`
	SubmittedBy                  string              `db:"submitted_by" json:"submitted_by"`
	TimeAdded                    time.Time           `db:"added_timestamp" json:"added_timestamp"`
	TimeProbed                   time.Time           `db:"probed_timestamp" json:"probed_timestamp"`
	IsOpen                       bool                `db:"is_open" json:"is_open"`
	RequestedReviewByDBReviewers bool                `db:"requested_review_by_db_reviewers" json:"requested_review_by_db_reviewers"`
	ApprovedByDBReviewers        bool                `db:"approved_by_db_reviewers" json:"approved_by_db_reviewers"`
	RequestedReviewByDBInfra     bool                `db:"requested_review_by_db_infra" json:"requested_review_by_db_infra"`
	ApprovedByDBInfra            bool                `db:"approved_by_db_infra" json:"approved_by_db_infra"`
	LabeledAsDiff                bool                `db:"label_diff" json:"label_diff"`
	LabeledAsDetected            bool                `db:"label_detected" json:"label_detected"`
	LabeledAsQueued              bool                `db:"label_queued" json:"label_queued"`
	LabeledForReview             bool                `db:"label_for_review" json:"label_for_review"`
}

func NewPullRequest() *PullRequest {
	return &PullRequest{
		Priority: PullRequestPriorityNormal,
		Status:   string(PullRequestStatusDetected),
	}
}

func NewPullRequestFromRepository(repo *Repository, number int) *PullRequest {
	return &PullRequest{
		Org:      repo.Org,
		Repo:     repo.Repo,
		Number:   number,
		Priority: PullRequestPriorityNormal,
		Status:   string(PullRequestStatusDetected),
	}
}

func (pr *PullRequest) GetStatus() PullRequestStatus {
	switch pr.Status {
	case string(PullRequestStatusDetected):
		return PullRequestStatusDetected
	case string(PullRequestStatusQueued):
		return PullRequestStatusQueued
	case string(PullRequestStatusCancelled):
		return PullRequestStatusCancelled
	case string(PullRequestStatusComplete):
		return PullRequestStatusComplete
	}
	return PullRequestStatusUnknown
}

func (pr *PullRequest) String() string {
	return fmt.Sprintf("%s/%s/pull/%d", pr.Org, pr.Repo, pr.Number)
}

func (pr *PullRequest) URL() string {
	return fmt.Sprintf("https://github.com/%s", pr.String())
}
