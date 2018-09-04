package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/spf13/viper"
)

func TestConfig(t *testing.T) {
	v := viper.New()
	configFile := "config.toml"
	v.SetConfigFile(configFile)
	err := v.ReadInConfig()
	if err != nil {
		t.Error(err)
	}
	config := &Config{}
	err = v.Unmarshal(config)
	if err != nil {
		t.Error(err)
	}
	v.WatchConfig()

	for _, key := range config.KeyStore.Keys {
		fmt.Printf("key:%s\n", key)
	}
	for _, node := range config.DgateWay.Nodes {
		fmt.Printf("testNode:%s,status:%t\n", node.Host, node.Status)
	}
	fmt.Printf("get:%s\n", config.DgateWay.NewNodeHost)
	fmt.Println("---------!")
	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Printf("name:%s\n", e.Name)
		v.Unmarshal(config)
	})
	go func() {
		ticker := time.NewTicker(time.Second).C
		for {
			<-ticker
			fmt.Println(config.DgateWay.NewNodeHost)
		}
	}()
	fmt.Println("---------")
	time.Sleep(120 * time.Second)
}
