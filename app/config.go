package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/spf13/viper"
	_ "github.com/spf13/viper/remote"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/connectivity"
)

type Config struct {
	Url    string `mapstruct:"url"`
	Dsn    string `mapstruct:"dsn"`
	Secret string `mapstruct:"secret"`
}

type ConfigHandler struct {
	host  string
	etcd  *clientv3.Client
	viper *viper.Viper
	paths []string
}

func NewConfigHandler(host string) *ConfigHandler {
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

	if err != nil {
		return nil
	}

	fmt.Printf("etcd > %s\n", etcd.ActiveConnection().GetState())

	return &ConfigHandler{
		host:  host,
		etcd:  etcd,
		viper: viper.New(),
	}
}

func (h *ConfigHandler) Write(ctx context.Context, name string, file []byte) error {
	_, err := h.etcd.Put(ctx, name, string(file))
	if err != nil {
		return err
	}

	return nil
}

func (h *ConfigHandler) DefaultConfig() (*Config, error) {
	config := new(Config)
	h.viper.SetConfigType("yaml")

	if err := h.viper.AddRemoteProvider("etcd3", h.host, "config/app.yaml"); err != nil {
		return nil, err
	}

	if err := h.viper.ReadRemoteConfig(); err != nil {
		fmt.Print(err.Error())

		return nil, err
	}

	if err := h.viper.Unmarshal(config); err != nil {
		return nil, err
	}

	return config, nil
}
