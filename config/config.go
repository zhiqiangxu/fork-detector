package config

import (
	"encoding/json"
	"io/ioutil"
)

type Config struct {
	Chains []Chain
}

func Init() {

}

type Chain struct {
	Type int
	URLs []string
	Name string
}

func LoadConfig(confFile string) (config *Config, err error) {
	jsonBytes, err := ioutil.ReadFile(confFile)
	if err != nil {
		return
	}

	config = &Config{}
	err = json.Unmarshal(jsonBytes, config)
	return
}
