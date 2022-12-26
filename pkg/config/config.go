package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	AllowNetworkList []string `mapstructure:"allowNetworkList,omitempty"`
	CountryAllowList []string `mapstructure:"countryAllowList,omitempty"`
	CountryDenyList  []string `mapstructure:"countryDenyList,omitempty"`
	RefreshInterval  string   `mapstructure:"refreshInterval,omitempty"`
	AppendDrop       bool     `mapstructure:"appendDrop,omitempty"`
}

// LoadConfig reads configuration from file or environment variables.
func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	return
}
