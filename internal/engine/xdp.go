package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"

	"github.com/cilium/ebpf"
)

const (
	// DeviceIDLen 设备ID固定长度（与 C 结构体保持一致）
	DeviceIDLen = 32
	// MaxPayloadSize 最大 Payload 大小（防止恶意超大包打爆内核 Map）
	MaxPayloadSize = 1024
)

// CmdPacket 对应内核态 C 结构体（必须严格内存对齐）
// 注意：Go 结构体中的字段顺序和大小必须与 C 语言完全一致
type CmdPacket struct {
	DeviceID   [DeviceIDLen]byte
	Payload    [MaxPayloadSize]byte
	PayloadLen uint32
}

type XDPEngine struct {
	coll        *ebpf.Collection
	cmdQueueMap *ebpf.Map
}

func InitXDPEngine() (*XDPEngine, error) {
	// 1. 定义用于接收 BPF 程序和 Map 的结构体
	// 注意：这里的字段名必须和 bpf2go 生成的结构体一致
	var objs XdpEngineObjects

	// 2. 直接调用生成的加载函数
	// 这个函数会自动从内存中读取嵌入的 .o 文件并加载到内核
	err := LoadXdpEngineObjects(&objs, nil)
	if err != nil {
		return nil, fmt.Errorf("loading XDP engine objects: %w", err)
	}

	// 3. 将加载好的对象赋值给您的引擎结构体
	engine := &XDPEngine{
		// 这里根据您实际的结构体字段进行赋值
		// 例如：
		// Progs: objs.XdpEngineProg,
		// Maps:  objs.XdpEngineMap,
	}

	return engine, nil
}

// // InitXDPEngine 加载 eBPF 程序并获取 Map 句柄
// func InitXDPEngine(objFile string) (*XDPEngine, error) {
// 	spec, err := ebpf.LoadCollectionSpec(objFile)
// 	if err != nil {
// 		return nil, fmt.Errorf("加载 eBPF 集合失败: %v", err)
// 	}

// 	coll, err := ebpf.NewCollection(spec)
// 	if err != nil {
// 		return nil, fmt.Errorf("创建 eBPF 集合失败: %v", err)
// 	}

// 	cmdQueue, ok := coll.Maps["cmd_queue"]
// 	if !ok {
// 		coll.Close()
// 		return nil, fmt.Errorf("未找到 cmd_queue Map")
// 	}

// 	return &XDPEngine{
// 		coll:        coll,
// 		cmdQueueMap: cmdQueue,
// 	}, nil
// }

// PushCommand 将车辆指令推送到内核态的 eBPF Map 中
func (e *XDPEngine) PushCommand(deviceID string, payload []byte) error {
	// 1. 校验 Payload 长度
	if len(payload) > MaxPayloadSize {
		return fmt.Errorf("payload 大小 %d 超过最大限制 %d", len(payload), MaxPayloadSize)
	}

	// 2. 构造严格对齐的内存包
	var pkt CmdPacket

	// 拷贝 DeviceID
	copy(pkt.DeviceID[:], deviceID)

	// 拷贝 Payload
	copy(pkt.Payload[:], payload)

	// 记录实际 Payload 长度
	pkt.PayloadLen = uint32(len(payload))

	// 3. 将结构体序列化为字节流（使用小端序，与 Linux 内核保持一致）
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, pkt); err != nil {
		return fmt.Errorf("序列化 CmdPacket 失败: %v", err)
	}

	// 4. 写入 eBPF Map (这里假设使用 Array Map，Key 为 0；如果是 Ringbuf 则使用 Push)
	// 注意：实际生产中，推荐使用 BPF_MAP_TYPE_RINGBUF 配合 cilium/ebpf/ringbuf 包
	key := uint32(0)
	if err := e.cmdQueueMap.Put(key, buf.Bytes()); err != nil {
		return fmt.Errorf("推送到 eBPF Map 失败: %v", err)
	}

	log.Printf("[XDPEngine] 成功下发指令至车辆 %s, Payload大小: %d bytes", deviceID, len(payload))
	return nil
}

// Close 安全卸载 eBPF 程序并释放资源
func (e *XDPEngine) Close() {
	if e.coll != nil {
		e.coll.Close()
		log.Println("[XDPEngine] eBPF 集合已安全卸载")
	}
}
