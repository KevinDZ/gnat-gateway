package engine

// 注意：这里的 xdp_engine 是生成文件的前缀，xdp_engine_kern.c 是内核态源码
// 注意这里增加了 -I/usr/include/x86_64-linux-gnu 或对应的内核头文件路径
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go XdpEngine ./xdp_engine_kern.c -- -I/usr/include/x86_64-linux-gnu -I../headers
