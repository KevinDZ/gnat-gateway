package hash

import (
	"fmt"
	"hash/crc32"
	"sort"
	"sync"
)

// ConsistentHash 一致性哈希结构体
type ConsistentHash struct {
	mu       sync.RWMutex
	replicas int               // 虚拟节点倍数
	ring     []uint32          // 排序后的哈希环
	hashMap  map[uint32]string // 哈希值到真实节点 IP 的映射
}

// NewConsistentHash 创建一致性哈希实例
// replicas: 每个真实节点对应的虚拟节点数量（推荐 100~200，保证数据分布均匀）
func NewConsistentHash(replicas int) *ConsistentHash {
	return &ConsistentHash{
		replicas: replicas,
		hashMap:  make(map[uint32]string),
	}
}

// Add 添加真实节点（会自动创建对应的虚拟节点）
func (c *ConsistentHash) Add(nodeIP string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 0; i < c.replicas; i++ {
		// 将真实节点与虚拟节点序号拼接后计算哈希值
		hash := crc32.ChecksumIEEE([]byte(fmt.Sprintf("%s#%d", nodeIP, i)))
		c.ring = append(c.ring, hash)
		c.hashMap[hash] = nodeIP
	}
	// 保持哈希环有序，以便后续二分查找
	sort.Slice(c.ring, func(i, j int) bool { return c.ring[i] < c.ring[j] })
}

// Remove 移除真实节点（节点宕机或缩容时使用）
func (c *ConsistentHash) Remove(nodeIP string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newRing := make([]uint32, 0)
	for i := 0; i < c.replicas; i++ {
		hash := crc32.ChecksumIEEE([]byte(fmt.Sprintf("%s#%d", nodeIP, i)))
		delete(c.hashMap, hash)
	}
	// 重建排序后的哈希环
	for hash := range c.hashMap {
		newRing = append(newRing, hash)
	}
	sort.Slice(newRing, func(i, j int) bool { return newRing[i] < newRing[j] })
	c.ring = newRing
}

// GetNode 根据设备 ID (如 VIN 码) 获取其所属的目标节点 IP
func (c *ConsistentHash) GetNode(deviceID string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.ring) == 0 {
		return "", fmt.Errorf("哈希环为空，无可用节点")
	}

	hash := crc32.ChecksumIEEE([]byte(deviceID))

	// 在有序环中二分查找第一个大于等于当前 hash 的节点
	idx := sort.Search(len(c.ring), func(i int) bool { return c.ring[i] >= hash })

	// 如果没找到，说明 hash 值超过了环的最大值，顺时针回绕到环的第一个节点
	if idx >= len(c.ring) {
		idx = 0
	}

	return c.hashMap[c.ring[idx]], nil
}

// GetNodes 获取当前哈希环上的所有真实节点（用于初始化或监控）
func (c *ConsistentHash) GetNodes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	nodeSet := make(map[string]struct{})
	for _, node := range c.hashMap {
		nodeSet[node] = struct{}{}
	}

	nodes := make([]string, 0, len(nodeSet))
	for node := range nodeSet {
		nodes = append(nodes, node)
	}
	return nodes
}

// ReplaceAll 全量替换哈希环上的节点（用于集群变更后的热更新）
func (c *ConsistentHash) ReplaceAll(nodeIPs []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 清空旧数据
	c.ring = make([]uint32, 0)
	c.hashMap = make(map[uint32]string)

	// 重新添加所有节点
	for _, nodeIP := range nodeIPs {
		for i := 0; i < c.replicas; i++ {
			hash := crc32.ChecksumIEEE([]byte(fmt.Sprintf("%s#%d", nodeIP, i)))
			c.ring = append(c.ring, hash)
			c.hashMap[hash] = nodeIP
		}
	}
	sort.Slice(c.ring, func(i, j int) bool { return c.ring[i] < c.ring[j] })
}
