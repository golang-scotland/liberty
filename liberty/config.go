package main

import (
	"fmt"

	"golang.scot/liberty/balancer"
	"golang.scot/liberty/middleware"

	"github.com/spf13/viper"
)

const (
	configFile     = "liberty"
	configLocation = "/etc/liberty"
)

var conf = loadConfig()

func loadConfig() *Config {
	cfg := &Config{}
	v := viper.New()
	v.SetConfigName(configFile)
	v.AddConfigPath(configLocation)
	err := v.ReadInConfig()
	if err != nil {
		fmt.Printf("Fatal error reading liberty config: %s\n", err)
	}
	v.Unmarshal(cfg)
	return cfg
}

func CurrentConfig() *Config {
	return conf
}

func SetConfig(cfg *Config) {
	conf = cfg
}

// Config is the top level configuration for this package, at this moment the
// persisted paramaters are expected to be read from a yaml file.
type Config struct {
	Env           string                     `yaml:"env"`
	Profiling     bool                       `yaml:"profiling"`
	ProfStatsFile string                     `yaml:"profStatsFile"`
	Certs         []*balancer.Crt            `yaml:"certs"`
	Proxies       []*middleware.Proxy        `yaml:"proxies"`
	Whitelist     []*middleware.ApiWhitelist `yaml:"whitelist"`
}
