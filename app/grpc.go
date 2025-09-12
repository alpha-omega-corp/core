package app

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
)

type Client[T any] struct {
	conn    grpc.ClientConnInterface
	service T
}

func NewClient[T any](target string, serviceConstructor func(conn grpc.ClientConnInterface) T) *Client[T] {
	conn, err := grpc.NewClient(target, grpc.WithInsecure())
	if err != nil {
		fmt.Printf("Could not connect to %v: %v", target, err)
	}

	return &Client[T]{
		service: serviceConstructor(conn),
		conn:    conn,
	}
}

func (c *Client[T]) Service() T {
	return c.service
}

func GRPC(address string, init func(grpc *grpc.Server)) error {
	listen, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	srv := grpc.NewServer()
	init(srv)

	fmt.Printf("running at tcp://%v\n", address)
	return srv.Serve(listen)
}
