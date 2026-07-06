package router

import (
	"context"
	"fmt"
	"log"
	"time"

	"gnat-gateway/internal/cache"
	"gnat-gateway/internal/engine"
	"gnat-gateway/internal/hash"
	"gnat-gateway/internal/pool"
	pb "gnat-gateway/internal/proto/internalpb"
)

type GNATGateway struct {
	nodeIP     string               // 当前网关节点的 IP
	hashRing   *hash.ConsistentHash // 一致性哈希环
	localCache *cache.LocalCache    // 本地 LRU 缓存
	grpcPool   *pool.GRPCPool       // gRPC 跨节点连接池
	xdpEngine  *engine.XDPEngine    // 新增 XDP 引擎
}

// NewGNATGateway 初始化网关核心组件
func NewGNATGateway(nodeIP string, hashRing *hash.ConsistentHash, cache *cache.LocalCache, pool *pool.GRPCPool, xdp *engine.XDPEngine) *GNATGateway {
	return &GNATGateway{
		nodeIP:     nodeIP,
		hashRing:   hashRing,
		localCache: cache,
		grpcPool:   pool,
		xdpEngine:  xdp, // NewGNATGateway 增加 xdpEngine 参数
	}
}

// RouteCommand 统一路由入口：自动判断是本地下发还是跨节点转发
func (g *GNATGateway) RouteCommand(ctx context.Context, deviceID, traceID string, payload []byte) error {
	// 1. 通过一致性哈希环，计算该车辆应该归属哪个节点
	targetIP, err := g.hashRing.GetNode(deviceID)
	if err != nil {
		return fmt.Errorf("获取目标节点失败: %v", err)
	}

	// 2. 判断目标节点是否为当前节点
	if targetIP == g.nodeIP {
		// 【本地处理】：车辆连接在当前节点，直接下发
		return g.handleLocalCommand(deviceID, payload)
	}

	// 3. 【跨节点转发】：车辆不在当前节点，通过 gRPC 转发
	log.Printf("[TraceID: %s] 车辆 %s 不在本节点，转发至节点 %s", traceID, deviceID, targetIP)
	return g.ForwardViaGRPC(ctx, targetIP, deviceID, traceID, payload)
}

// handleLocalCommand 本地车辆指令下发逻辑
func (g *GNATGateway) handleLocalCommand(deviceID string, payload []byte) error {
	// TODO: 在这里对接底层的 XDP/eBPF 引擎或 TCP/MQTT 连接池，将 payload 发送给车辆
	log.Printf("本地下发指令至车辆 %s, 经由 XDP 引擎, Payload大小: %d bytes", deviceID, len(payload))

	if g.xdpEngine == nil {
		return fmt.Errorf("XDP 引擎未初始化")
	}

	// 将指令推送到 eBPF Map，由内核态接管后续的网络发包
	if err := g.xdpEngine.PushCommand(deviceID, payload); err != nil {
		return fmt.Errorf("推送到 XDP 引擎失败: %v", err)
	}

	return nil
}

// ForwardViaGRPC 跨节点转发指令
func (g *GNATGateway) ForwardViaGRPC(ctx context.Context, targetIP, deviceID, traceID string, payload []byte) error {
	// 1. 从连接池获取目标节点的 gRPC 连接
	conn, err := g.grpcPool.GetConn(targetIP)
	if err != nil {
		return err
	}

	// 2. 创建 gRPC 客户端并发起 RPC 调用
	client := pb.NewInternalRouterClient(conn)
	resp, err := client.RelayCommand(ctx, &pb.RelayRequest{
		TraceId:  traceID,
		DeviceId: deviceID,
		Payload:  payload,
		ExpireAt: time.Now().Add(5 * time.Second).UnixMilli(), // 设置 5 秒超时
	})
	if err != nil {
		return fmt.Errorf("跨节点 RPC 调用失败: %v", err)
	}

	// 3. 检查目标节点的处理结果
	if !resp.Success {
		return fmt.Errorf("目标节点处理失败: %s", resp.Message)
	}
	return nil
}
