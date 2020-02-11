package core

import (
	"time"
)

type RepositoryProductionMapping struct {
	Id           int64  `db:"id" json:"id"`
	Org          string `db:"org" json:"org"`
	Repo         string `db:"repo" json:"repo"`
	Hint         string `db:"hint" json:"hint"`
	MySQLCluster string `db:"mysql_cluster" json:"mysql_cluster"`
	MySQLSchema  string `db:"mysql_schema" json:"mysql_schema"`

	TimeAdded   time.Time `db:"added_timestamp" json:"added_timestamp"`
	TimeUpdated time.Time `db:"updated_timestamp" json:"updated_timestamp"`
}

func NewRepositoryProductionMapping() *RepositoryProductionMapping {
	return &RepositoryProductionMapping{}
}

func NewRepositoryProductionMappingFromRepo(r *Repository) *RepositoryProductionMapping {
	return &RepositoryProductionMapping{
		Org:  r.Org,
		Repo: r.Repo,
	}
}
