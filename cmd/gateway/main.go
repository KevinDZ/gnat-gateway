package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gnat-gateway/internal/cache"
	"gnat-gateway/internal/config"
	"gnat-gateway/internal/engine"
	"gnat-gateway/internal/hash"
	"gnat-gateway/internal/pool"
	"gnat-gateway/internal/router"

	"github.com/redis/go-redis/v9"
)

var fileFlag = flag.String("file", "configs/config.yaml", "配置文件路径")

func main() {
	flag.Parse()
	// 0. 初始化配置
	cfg := config.InitConfig(*fileFlag)
	// 1. 初始化本地 LRU 缓存
	localCache, err := cache.NewLocalCache(cfg.CacheSize)
	if err != nil {
		log.Fatalf("初始化 LRU 缓存失败: %v", err)
	}

	// 2. 初始化 gRPC 跨节点连接池
	grpcPool := pool.NewGRPCPool()

	// 3. 初始化一致性哈希环，并加入集群节点
	hashRing := hash.NewConsistentHash(cfg.Replicas) // 150 个虚拟节点
	// hashRing.Add("192.168.1.100")           // 当前节点
	// hashRing.Add("192.168.1.101")           // 集群中的其他节点
	// hashRing.Add("192.168.1.102")

	// 3. 初始化 XDP/eBPF 引擎
	// 注意：objFile 是编译后的 eBPF 字节码文件路径，例如 "xdp_engine_kern.o"
	// xdpEngine, err := engine.InitXDPEngine("xdp_engine_kern.o")
	// if err != nil {
	// 	log.Fatalf("初始化 XDP 引擎失败: %v", err)
	// }
	// defer xdpEngine.Close() // 确保程序退出时释放 eBPF 资源
	// 之前的代码：xdpEngine, err := engine.InitXDPEngine("xdp_engine_kern.o")
	// 修改为：
	xdpEngine, err := engine.InitXDPEngine()
	if err != nil {
		log.Fatalf("初始化 XDP 引擎失败: %v", err)
	}
	defer xdpEngine.Close()

	// 4. 获取当前节点运行的进程数量
	runningNumber := cfg.GetRunningNumberBash()
	log.Printf("当前节点运行的节点数量: %d", runningNumber)
	currentNodeIP := fmt.Sprintf(cfg.CurrentNode, runningNumber)
	// currentNodeIP := cfg.CurrentNode // fmt.Sprintf(cfg.CurrentNode, runningNumber)

	// 加入redis集群哈希环
	// 2. 初始化 Redis 客户端
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.Redis.Addr, // 替换为真实的 Redis 地址
	})
	defer rdb.Close()

	// 3. 启动集群哈希环同步器
	syncer := hash.NewRedisSync(currentNodeIP, rdb, hashRing)
	if err := syncer.Start(); err != nil {
		log.Fatalf("启动 Redis 同步失败: %v", err)
	}

	// 4. 初始化网关核心
	gnat := router.NewGNATGateway(currentNodeIP, hashRing, localCache, grpcPool, xdpEngine)
	_ = gnat

	// 5. 优雅退出：监听系统信号，确保节点下线时能正确从 Redis 移除自己
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gateway...")
	syncer.Stop() // 关键：安全下线
}
