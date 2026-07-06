#!/bin/bash
# 安装生成 Go 基础代码的插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
# 安装生成 gRPC 服务代码的插件
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest


# internal_router.proto 保存在当前目录中
buf config init
buf dep update

# buf.gen.yaml 将放置在当前目录中
buf generate
