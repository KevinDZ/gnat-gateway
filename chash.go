package gnat

import (
	"fmt"
	"hash/crc32"
	"sort"
	"sync"
)

// ConsistentHash 一致性哈希结构
type ConsistentHash struct {
	mu         sync.RWMutex
	replicas   int               // 每个物理节点对应的虚拟节点数量
	ring       []uint32          // 排序后的哈希环
	hashMap    map[uint32]string // 哈希值 -> 物理节点标识 (如 IP:Port)
	nodeStatus map[string]bool   // 物理节点是否在线
}

// NewConsistentHash 创建一致性哈希实例
func NewConsistentHash(replicas int) *ConsistentHash {
	return &ConsistentHash{
		replicas:   replicas,
		hashMap:    make(map[uint32]string),
		nodeStatus: make(map[string]bool),
	}
}

// hashKey 使用 CRC32 生成哈希值
func (c *ConsistentHash) hashKey(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
}

// AddNode 添加物理节点
func (c *ConsistentHash) AddNode(node string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.nodeStatus[node] {
		return // 节点已存在
	}

	// 创建虚拟节点并加入环中
	for i := 0; i < c.replicas; i++ {
		vNodeKey := fmt.Sprintf("%s#%d", node, i)
		h := c.hashKey(vNodeKey)
		c.ring = append(c.ring, h)
		c.hashMap[h] = node
	}
	sort.Slice(c.ring, func(i, j int) bool { return c.ring[i] < c.ring[j] })
	c.nodeStatus[node] = true
}

// RemoveNode 移除物理节点 (例如节点宕机)
func (c *ConsistentHash) RemoveNode(node string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.nodeStatus[node] {
		return
	}

	// 从环中移除该节点的所有虚拟节点
	newRing := make([]uint32, 0)
	for _, h := range c.ring {
		if c.hashMap[h] != node {
			newRing = append(newRing, h)
		} else {
			delete(c.hashMap, h)
		}
	}
	c.ring = newRing
	delete(c.nodeStatus, node)
}

// GetNode 根据 Key (如 VIN 码) 获取对应的物理节点
func (c *ConsistentHash) GetNode(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.ring) == 0 {
		return ""
	}

	h := c.hashKey(key)
	// 二分查找，找到哈希环上顺时针方向的第一个节点
	idx := sort.Search(len(c.ring), func(i int) bool { return c.ring[i] >= h })
	if idx >= len(c.ring) {
		idx = 0 // 如果超过环的最大值，则绕回起点
	}
	return c.hashMap[c.ring[idx]]
}
