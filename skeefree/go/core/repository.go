package core

import (
	"fmt"
	"time"
)

type Repository struct {
	Id           int64  `db:"id" json:"id"`
	Org          string `db:"org" json:"org"`
	Repo         string `db:"repo" json:"repo"`
	Owner        string `db:"owner" json:"owner"`
	AutoRun      bool   `db:"autorun" json:"autorun"`
	MySQLCluster string
	MySQLSchema  string

	TimeAdded   time.Time `db:"added_timestamp" json:"added_timestamp"`
	TimeUpdated time.Time `db:"updated_timestamp" json:"updated_timestamp"`
}

func NewRepository(id int64) *Repository {
	return &Repository{
		Id: id,
	}
}
func NewRepositoryFromPullRequest(pr *PullRequest) *Repository {
	return &Repository{
		Org:  pr.Org,
		Repo: pr.Repo,
	}
}

func (r *Repository) HasOrgRepo() bool {
	return r.Org != "" && r.Repo != ""
}

func (r *Repository) OrgRepo() string {
	return fmt.Sprintf("%s/%s", r.Org, r.Repo)
}
