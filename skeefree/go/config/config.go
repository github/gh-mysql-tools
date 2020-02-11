package config

// Config is the app-specific configuration definition for the skeefree
// application.
//
// It is read from the environment variables present in the container/ on which
// this application is run, and initialized using the 'config' package
//
type Config struct {
	MysqlRwHost string `config:",env=SKEEFREE_MYSQL_HOST,127.0.0.1"` // Backend host used by this application
	MysqlRwUser string `config:",env=SKEEFREE_MYSQL_USER,root"`
	MysqlRwPass string `config:",env=SKEEFREE_MYSQL_PASS,"`
	MysqlSchema string `config:",env=SKEEFREE_MYSQL_SCHEMA,required"`

	SkeefreeDDLUser string `config:",env=SKEEFREE_DDL_USER,required"` // Account that has DDL privileges on all production servers
	SkeefreeDDLPass string `config:",env=SKEEFREE_DDL_PASS,required"`

	GitHubAPIToken       string `config:",env=HUBOT_GITHUB_TOKEN,required"`      // This token will be used by skeefree to examine your org, teams, and manipulate your PRs.
	SitesAPIPassword     string `config:",env=SITES_API_PASSWORD,required"`      // Internal GitHub inventory service. Not included in this OSS release.
	SitesAPIUrl          string `config:",env=SITES_API_URL,required"`           // Internal GitHub inventory service. Not included in this OSS release.
	MySQLDiscoveryAPIUrl string `config:",env=MYSQL_DISCOVERY_API_URL,required"` // Internal GitHub MySQL invenstory service

	DefaultOrg string `config:"name-of-my-org"` // Currently a single org is supported

	DBInfra     string `config:"name-of-database-team"`  // team should exist in your GitHub org
	DBReviewers string `config:"name-of-reviewers-team"` // team should exist in your GitHub org
}
