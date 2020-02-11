package gh

// SitesAPI is an internal GitHub inventory service.
// Each host has some general properties (such as `site`) and possible specific properties (aka attributes or tags).
// Of those, we are interested in `mysql_cluster` and `mysql_shard`
// e.g. you may ask it to list hosts that belong to a certain cluster, e.g. have `mysql_cluster=mycluster`
// and you may then e.g. look for all `mysql_shard` values.

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/github/skeefree/go/config"
)

type Instance struct {
	Hostname   string `json:"hostname"`
	Site       string `json:"site"`
	Attributes struct {
		MySQLCluster string `json:"github:mysql_cluster"`
		MySQLRole    string `json:"github:mysql_role"`
		MySQLShard   string `json:"github:mysql_shard"`
	} `json:"attributes"`
}

type SitesAPI struct {
	httpClient *http.Client
	password   string
	url        string
}

func NewSitesAPI(c *config.Config) (*SitesAPI, error) {
	httpClient, err := setupHttpClient()
	if err != nil {
		return nil, err
	}
	return &SitesAPI{
		httpClient: httpClient,
		password:   c.SitesAPIPassword,
		url:        c.SitesAPIUrl,
	}, nil
}

func (api *SitesAPI) get(path string, params url.Values) (resp *http.Response, err error) {
	url := fmt.Sprintf("%s/%s", api.url, path)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	request.URL.RawQuery = params.Encode()
	request.SetBasicAuth("x", api.password)
	return api.httpClient.Do(request)
}

func (api *SitesAPI) GetInstance(hostname string) (instance *Instance, err error) {
	resp, err := api.get(fmt.Sprintf("instances/%s", hostname), url.Values{})

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return instance, err
	}

	if err := json.Unmarshal(b, &instance); err != nil {
		return instance, err
	}
	return instance, err
}

func (api *SitesAPI) getInstances(params url.Values) (resp *http.Response, err error) {
	return api.get("instances", params)
}

func (api *SitesAPI) instances(params url.Values) (instances []Instance, err error) {
	resp, err := api.getInstances(params)

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return instances, err
	}

	if err := json.Unmarshal(b, &instances); err != nil {
		return instances, err
	}
	return instances, err
}

func (api *SitesAPI) ValidateMySQLCluster(mysqlCluster string) (instances []Instance, err error) {
	instances, err = api.instances(url.Values{"mysql_cluster": {mysqlCluster}})
	if err != nil {
		return instances, err
	}
	if len(instances) == 0 {
		return instances, fmt.Errorf("No instances found for mysql_cluster=%s", mysqlCluster)
	}
	return instances, err
}

// MySQLClusterShards returns a list of shard names for the given cluster.
// If the cluster is not sharded, the result is []string{""}
func (api *SitesAPI) MySQLClusterShards(mysqlCluster string) (shards []string, err error) {
	instances, err := api.ValidateMySQLCluster(mysqlCluster)
	if err != nil {
		return shards, err
	}
	shardsMap := make(map[string]bool)
	for _, instance := range instances {
		shardsMap[instance.Attributes.MySQLShard] = true
	}
	for shard := range shardsMap {
		shards = append(shards, shard)
	}
	return shards, nil
}
