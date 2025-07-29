package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/connectivity"
	"log"
	"time"
)

var Environment = map[string]string{
	"local":  "local",
	"docker": "docker",
}

func GetConfigPath(env string) string {
	path := "config/config." + Environment[env] + ".yml"

	return path
}

type Config struct {
	Url *string `mapstruct:"url"`
	Dsn *string `mapstruct:"dsn"`
	Env *viper.Viper
}

type ConfigHandler struct {
	name   string
	viper  *viper.Viper
	etcd   *clientv3.Client
	config *Config
}

func NewConfigHandler(ctx context.Context, name string, file []byte) *ConfigHandler {
	v := viper.New()

	v.SetConfigType("yaml")
	err := v.ReadConfig(bytes.NewBuffer(file))
	if err != nil {
		panic(err)
	}

	host := v.GetString("kvs")
	fmt.Printf("config host > %s\n", host)

	config := clientv3.Config{
		Endpoints:   []string{host},
		DialTimeout: 2 * time.Second,
	}

	etcd, err := clientv3.New(config)
	if err != nil {
		panic(err)
	}

	cancelCtx, cancel := context.WithTimeout(context.Background(), config.DialTimeout)
	defer cancel()

	select {
	case <-cancelCtx.Done():
		if etcd.ActiveConnection().GetState() != connectivity.Ready {
			log.Fatalf("etcd connection timeout")
		}
	}

	fmt.Printf("etcd > %s\n", etcd.ActiveConnection().GetState())

	_, err = etcd.Put(ctx, "config_"+name, string(file))
	if err != nil {
		panic(err)
	}

	handler := &ConfigHandler{
		etcd:  etcd,
		viper: viper.New(),
	}

	c, err := handler.requestConfig(name)
	if err != nil {
		panic(err)
	}

	handler.config = c

	return handler
}

func (h *ConfigHandler) GetConfig() *Config {
	return h.config
}

func (h *ConfigHandler) WithConfig(name string) *Config {
	config, err := h.requestConfig(name)
	if err != nil {
		log.Fatalf("no configuration found for application: %v\n%v", name, err.Error())
	}

	return config
}

func (h *ConfigHandler) requestConfig(name string) (config *Config, err error) {
	err = h.get("config_"+name, "yaml")
	if err != nil {
		log.Fatalf("no configuration found for application: %v\n%v", name, err.Error())
	}

	err = h.viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}

	config.Env = h.viper

	return config, nil
}

func (h *ConfigHandler) get(key string, format string) (err error) {
	h.viper = viper.New()

	host := h.etcd.Endpoints()[0]

	err = h.viper.AddRemoteProvider("etcd3", "http://"+host, key)
	if err != nil {
		return err
	}

	h.viper.SetConfigType(format)
	err = h.viper.ReadRemoteConfig()

	return
}

func deepCopyConfig(original *ConfigHandler) (*ConfigHandler, error) {
	data, err := json.Marshal(original)
	if err != nil {
		return nil, err
	}

	var duplicate ConfigHandler
	err = json.Unmarshal(data, &duplicate)
	if err != nil {
		return nil, err
	}

	return &duplicate, nil
}
