package pool

import (
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPCPool struct {
	mu    sync.RWMutex
	conns map[string]*grpc.ClientConn
}

func NewGRPCPool() *GRPCPool {
	return &GRPCPool{conns: make(map[string]*grpc.ClientConn)}
}

func (p *GRPCPool) GetConn(targetIP string) (*grpc.ClientConn, error) {
	p.mu.RLock()
	if conn, ok := p.conns[targetIP]; ok {
		p.mu.RUnlock()
		return conn, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()
	if conn, ok := p.conns[targetIP]; ok {
		return conn, nil
	}

	conn, err := grpc.NewClient(
		targetIP,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(3*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 gRPC 连接失败: %v", err)
	}
	p.conns[targetIP] = conn
	return conn, nil
}
