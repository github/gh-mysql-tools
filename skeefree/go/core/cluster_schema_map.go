package core

type MySQLClusterSchema struct {
	MySQLCluster string `db:"mysql_cluster" json:"mysql_cluster"`
	MySQLSchema  string `db:"mysql_schema" json:"mysql_schema"`
}

func NewMySQLClusterSchema(cluster string, schema string) *MySQLClusterSchema {
	return &MySQLClusterSchema{
		MySQLCluster: cluster,
		MySQLSchema:  schema,
	}
}

type MySQLClusterSchemaMap map[string](*MySQLClusterSchema)
