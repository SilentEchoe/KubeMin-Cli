package config

import (
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"encoding/json"
	v1 "k8s.io/api/core/v1"
)

const (
	Mysql = "mysql"
)

type Config struct {
	ConfigMapName string //判断是从configmap中获取配置还是从文件中获取配置
	ConfigInfo    ConfigInfo
}

type ConfigInfo struct {
	BinAddr       string
	Datastore     datastore.Config
	DatastoreType string
	LocalCluster  bool
	IstioEnable   bool
}

func NewConfig() *Config {
	return &Config{
		ConfigMapName: "", // 默认是空，如果想使用config 则填写configmap的名字
		ConfigInfo: ConfigInfo{
			BinAddr: "0.0.0.0:8000",
			Datastore: datastore.Config{
				Type:     "kubeapi",
				Database: "kubemincli",
				URL:      "Data Source=127.0.0.1;Database=kubemin;User Id=root;Password=123456;",
			},
			LocalCluster:  true,
			DatastoreType: Mysql,
			IstioEnable:   false,
		},
	}
}

func (c *Config) ParseConfigMap(maps *v1.ConfigMap) error {

	if maps != nil {
		data, err := json.Marshal(maps.Data)
		if err != nil {
			return err
		}

		err = json.Unmarshal(data, &c.ConfigInfo)
		if err != nil {
			return err
		}
	}

	return error(nil)
}
