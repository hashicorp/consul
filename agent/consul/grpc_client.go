package consul

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/pool"
	"google.golang.org/grpc"
)

const (
	grpcBasePath = "/consul"
)

func dialGRPC(addr string, _ time.Duration) (net.Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	_, err = conn.Write([]byte{pool.RPCGRPC})
	if err != nil {
		return nil, err
	}

	return conn, nil
}

type GRPCClient struct {
	grpcConns     map[string]*grpc.ClientConn
	grpcConnsLock sync.RWMutex
	logger        *log.Logger
}

func NewGRPCClient(logger *log.Logger) *GRPCClient {
	return &GRPCClient{
		grpcConns: make(map[string]*grpc.ClientConn),
		logger:    logger,
	}
}

func (c *GRPCClient) Call(dc string, server *metadata.Server, method string, args, reply interface{}) error {
	conn, err := c.grpcConn(server)
	if err != nil {
		return err
	}

	c.logger.Printf("[TRACE] Using GRPC for method %s", method)
	return conn.Invoke(context.Background(), c.grpcPath(method), args, reply)
}

func (c *GRPCClient) grpcConn(server *metadata.Server) (*grpc.ClientConn, error) {
	host, _, _ := net.SplitHostPort(server.Addr.String())
	addr := fmt.Sprintf("%s:%d", host, server.Port)

	c.grpcConnsLock.RLock()
	conn, ok := c.grpcConns[addr]
	c.grpcConnsLock.RUnlock()
	if ok {
		return conn, nil
	}

	c.grpcConnsLock.Lock()
	defer c.grpcConnsLock.Unlock()

	co, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithDialer(dialGRPC))
	if err != nil {
		return nil, err
	}

	c.grpcConns[addr] = conn
	return co, nil
}

func (c *GRPCClient) grpcPath(p string) string {
	return grpcBasePath + "." + strings.Replace(p, ".", "/", -1)
}
