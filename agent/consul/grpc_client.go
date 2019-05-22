package consul

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/structs"
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
	routers       *router.Manager
	logger        *log.Logger
}

func NewGRPCClient(logger *log.Logger, routers *router.Manager) *GRPCClient {
	return &GRPCClient{
		grpcConns: make(map[string]*grpc.ClientConn),
		routers:   routers,
		logger:    logger,
	}
}

func (c *GRPCClient) GRPCConn() (*grpc.ClientConn, error) {
	server := c.routers.FindServer()
	if server == nil {
		return nil, structs.ErrNoServers
	}

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
