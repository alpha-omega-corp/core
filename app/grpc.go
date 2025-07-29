package app

import (
	"fmt"
	"github.com/uptrace/bun"
	"google.golang.org/grpc"
	"log"
	"net"
)

func GRPC(configHandler ConfigHandler, dbHandler *StorageHandler, init func(db *bun.DB, grpc *grpc.Server)) error {
	config := configHandler.config

	listen, err := net.Listen("tcp", *config.Url)

	if err != nil {
		return err
	}

	srv := grpc.NewServer()
	if dbHandler != nil {
		db := dbHandler.Database()
		defer func(db *bun.DB) {
			err := db.Close()
			if err != nil {
				log.Fatal(err)
			}
		}(db)

		init(db, srv)
	} else {
		init(nil, srv)
	}

	fmt.Printf("running at tcp://%v", *config.Url)
	return srv.Serve(listen)
}

func NewClient[T any](c *Config, proto func(conn grpc.ClientConnInterface) T) T {
	conn, err := grpc.NewClient(*c.Url, grpc.WithInsecure())

	if err != nil {
		fmt.Printf("Could not connect to %v: %v", *c.Url, err)
	}

	return proto(conn)
}
