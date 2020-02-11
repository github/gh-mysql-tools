package db

import (
	"fmt"
	"time"

	"github.com/github/skeefree/go/config"
	"github.com/github/skeefree/go/core"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// TopologyDB applies schema migrations directly on production masters

type TopologyDB struct {
	db        *sqlx.DB
	statement string
}

func NewTopologyDB(c *config.Config, migration *core.Migration) (*TopologyDB, error) {
	db, err := sqlx.Open("mysql", mysqlProductionMasterDSN(c, migration))
	if err != nil {
		return nil, err
	}
	return &TopologyDB{
		db:        db,
		statement: migration.PRStatement.Statement,
	}, nil
}

func mysqlProductionMasterDSN(c *config.Config, migration *core.Migration) string {
	cfg := mysql.NewConfig()
	cfg.User = c.SkeefreeDDLUser
	cfg.Passwd = c.SkeefreeDDLPass
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", migration.Cluster.RWName, migration.Cluster.Port)
	cfg.DBName = migration.Repo.MySQLSchema
	cfg.ParseTime = true
	cfg.InterpolateParams = true
	cfg.Timeout = 10 * time.Second

	return cfg.FormatDSN()
}

func (topology *TopologyDB) Get(dest interface{}, query string, args ...interface{}) (err error) {
	return topology.db.Get(dest, query, args...)
}

func (topology *TopologyDB) Exec(query string) (rowsAffected int64, err error) {
	result, err := topology.db.Exec(query)
	if err != nil {
		return rowsAffected, err
	}
	return result.RowsAffected()
}

func (topology *TopologyDB) Ping() (readOnly bool, err error) {
	err = topology.db.Get(&readOnly, "select @@global.read_only")
	return readOnly, err
}
