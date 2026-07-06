#!/bin/bash

sudo apt update
sudo apt install -y llvm clang
llvm-strip --version

# 先找到 llvm-strip 的实际路径
which llvm-strip-14 
# 创建软链接（假设实际路径是 /usr/bin/llvm-strip-14）
sudo ln -s /usr/bin/llvm-strip-14 /usr/bin/llvm-strip

# c文件编译 o可执行文件
# o执行文件 编译 go文件
# //go:embed 将 .o 字节码直接嵌入到最终的 Go 二进制文件中
go generate
