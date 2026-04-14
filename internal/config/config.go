package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	ClusterName    string
	NetworkName    string
	Subnet         string
	ClusterImage   string
	BaseDisk       string
	KubeconfigPath string
	DefaultMemory  int
	DefaultVCPUs   int
	Verbose        bool
	Debug          bool
}

func Load() (*Config, error) {
	viper.SetDefault("cluster.name", DefaultNetworkName)
	viper.SetDefault("cluster.network_name", DefaultNetworkName)
	viper.SetDefault("cluster.subnet", DefaultSubnet)
	viper.SetDefault("cluster.image", DefaultClusterImage)
	viper.SetDefault("cluster.base_disk", DefaultBaseDisk)
	viper.SetDefault("cluster.kubeconfig_path", fmt.Sprintf("%s/kubeconfig", DefaultKubeconfigDir))
	viper.SetDefault("node.memory", DefaultMemory)
	viper.SetDefault("node.vcpus", DefaultVCPUs)
	viper.SetDefault("logging.verbose", false)
	viper.SetDefault("logging.debug", false)

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.bink")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("BINK")
	viper.AutomaticEnv()

	_ = viper.ReadInConfig()

	cfg := &Config{
		ClusterName:    viper.GetString("cluster.name"),
		NetworkName:    viper.GetString("cluster.network_name"),
		Subnet:         viper.GetString("cluster.subnet"),
		ClusterImage:   viper.GetString("cluster.image"),
		BaseDisk:       viper.GetString("cluster.base_disk"),
		KubeconfigPath: viper.GetString("cluster.kubeconfig_path"),
		DefaultMemory:  viper.GetInt("node.memory"),
		DefaultVCPUs:   viper.GetInt("node.vcpus"),
		Verbose:        viper.GetBool("logging.verbose"),
		Debug:          viper.GetBool("logging.debug"),
	}

	return cfg, nil
}
