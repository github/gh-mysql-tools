package gh

// mysql-discovery is an internal GitHub service, a MySQL-specific inventory.
// The general purpose of the service is to provide information about clusters:
// - find a cluster by name(s)
// - find the RW and RO DNS names for the cluster
// - Which port do instances of this cluster listen on?
// - Is this cluster behind Vitess?

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/github/skeefree/go/config"

	"github.com/patrickmn/go-cache"
)

// teamMembers cache: key=cluster name; value = *MySQLCluster
var clusterCache = cache.New(time.Hour, 10*time.Minute)

type MySQLCluster struct {
	Name                   string `db:"cluster_name" yaml:"cluster_name" json:"cluster_name" `
	AlternateName          string `db:"alternate_name" yaml:"alternate_name" json:"alternate_name" `
	RWName                 string `db:"rw_name" yaml:"rw_name" json:"rw_name" `
	ROName                 string `db:"ro_name" yaml:"ro_name" json:"ro_name" `
	AnalyticsName          string `db:"analytics_name" yaml:"analytics_name" json:"analytics_name" `
	GLBPoolName            string `db:"glb_pool_name" yaml:"glb_pool_name" json:"glb_pool_name" `
	State                  string `db:"state" yaml:"state" json:"state" `
	DBConsoleName          string // evaluated from other fields
	DBConsoleDefaultSchema string `db:"dbconsole_default_schema" yaml:"dbconsole_default_schema" json:"dbconsole_default_schema" `
	Port                   int    `db:"port" yaml:"port" json:"port" `
	IsVitess               bool   `db:"vitess" yaml:"vitess" json:"vitess" `

	TimeUpdated time.Time `db:"time_updated" json:"time_updated" `
}

type MySQLDiscoveryAPI struct {
	httpClient *http.Client
	url        string
}

func NewMySQLDiscoveryAPI(c *config.Config) (*MySQLDiscoveryAPI, error) {
	httpClient, err := setupHttpClient()
	if err != nil {
		return nil, err
	}
	return &MySQLDiscoveryAPI{
		url:        c.MySQLDiscoveryAPIUrl,
		httpClient: httpClient,
	}, nil
}

func (api *MySQLDiscoveryAPI) get(path string) (resp *http.Response, err error) {
	url := fmt.Sprintf("%s/%s", api.url, path)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return api.httpClient.Do(request)
}

func (api *MySQLDiscoveryAPI) GetCluster(clusterName string) (cluster *MySQLCluster, err error) {
	if item, found := clusterCache.Get(clusterName); found {
		cluster = item.(*MySQLCluster)
		return cluster, nil
	}

	resp, err := api.get(fmt.Sprintf("cluster/%s", clusterName))

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return cluster, err
	}

	if err := json.Unmarshal(b, &cluster); err != nil {
		return cluster, err
	}
	clusterCache.SetDefault(clusterName, cluster)
	return cluster, err
}
