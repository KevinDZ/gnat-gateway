package hash

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Redis Set Key，用于存储当前所有存活的网关节点 IP
	NodeSetKey = "gnat:gateway:nodes"
	// Redis Pub/Sub Channel，用于广播节点上下线事件
	NodeEventChannel = "gnat:gateway:node_events"
)

// NodeEvent 节点变更事件
type NodeEvent struct {
	Type   string `json:"type"`    // "join" 或 "leave"
	NodeIP string `json:"node_ip"` // 变更的节点 IP
	TS     int64  `json:"ts"`      // 事件时间戳
}

// RedisSync 负责将一致性哈希与 Redis 集群同步
type RedisSync struct {
	nodeIP   string
	rdb      *redis.Client
	hashRing *ConsistentHash
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewRedisSync 创建同步器
func NewRedisSync(nodeIP string, rdb *redis.Client, hashRing *ConsistentHash) *RedisSync {
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisSync{
		nodeIP:   nodeIP,
		rdb:      rdb,
		hashRing: hashRing,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动同步器（节点上线）
func (s *RedisSync) Start() error {

	// 1. 将自己加入集群节点集合
	if err := s.rdb.SAdd(s.ctx, NodeSetKey, s.nodeIP).Err(); err != nil {
		return fmt.Errorf("注册节点到 Redis 失败: %v", err)
	}

	// 2. 发布节点加入事件
	s.publishEvent("join")

	// 3. 启动后台协程，订阅节点变更事件
	go s.subscribeEvents()

	log.Printf("[RedisSync] 节点 %s 已上线并加入集群", s.nodeIP)
	return nil
}

// Stop 停止同步器（节点优雅下线）
func (s *RedisSync) Stop() {
	// 1. 从集合中移除自己
	s.rdb.SRem(s.ctx, NodeSetKey, s.nodeIP)
	// 2. 发布节点离开事件
	s.publishEvent("leave")
	// 3. 取消订阅协程
	s.cancel()
	log.Printf("[RedisSync] 节点 %s 已安全下线", s.nodeIP)
}

// publishEvent 发布节点变更事件
func (s *RedisSync) publishEvent(eventType string) {
	event := NodeEvent{
		Type:   eventType,
		NodeIP: s.nodeIP,
		TS:     time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(event)
	if err := s.rdb.Publish(s.ctx, NodeEventChannel, data).Err(); err != nil {
		log.Printf("[RedisSync] 发布事件失败: %v", err)
	}
}

// subscribeEvents 监听集群节点变更
func (s *RedisSync) subscribeEvents() {
	sub := s.rdb.Subscribe(s.ctx, NodeEventChannel)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-s.ctx.Done():
			return
		case msg := <-ch:
			var event NodeEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("[RedisSync] 解析事件失败: %v", err)
				continue
			}
			// 收到事件后，从 Redis 拉取最新节点列表，重建本地哈希环
			s.rebuildHashRing()
		}
	}
}

// raebuildHshRing 从 Redis 拉取最新节点列表，热更新哈希环
func (s *RedisSync) rebuildHashRing() {
	nodes, err := s.rdb.SMembers(s.ctx, NodeSetKey).Result()
	if err != nil {
		log.Printf("[RedisSync] 拉取节点列表失败: %v", err)
		return
	}

	// 调用一致性哈希的 ReplaceAll 方法（需在 consistent_hash.go 中补充此方法）
	s.hashRing.ReplaceAll(nodes)
	log.Printf("[RedisSync] 哈希环已热更新，当前存活节点数: %d", len(nodes))
}
