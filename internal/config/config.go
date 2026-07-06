package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"

	"go.yaml.in/yaml/v2"
)

type Config struct {
	CurrentNode string `yaml:"current_node"`
	Redis       struct {
		Addr string `yaml:"addr"`
	}
	CacheSize   int    `yaml:"cache_size"`
	Replicas    int    `yaml:"replicas"`
	NatstatBash string `yaml:"natstat_bash"`
	Awk7Bash    string `yaml:"awk7_bash"`
	NameBash    string `yaml:"name_bash"`
}

func InitConfig(conf string) *Config {
	// 1. 读取文件内容（Go 1.16+ 推荐使用 os.ReadFile）
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("获取当前工作目录失败: %v", err)
	}
	conf = dir + "/" + conf
	yamlFile, err := os.ReadFile(conf)
	if err != nil {
		log.Fatalf("读取文件失败: %v", err)
	}

	// 2. 解析 YAML 数据到结构体
	var cfg Config
	err = yaml.Unmarshal(yamlFile, &cfg)
	if err != nil {
		log.Fatalf("解析 YAML 失败: %v", err)
	}
	return &cfg
}

// sort -u (排序并去重) <-- 新增步骤
func (c *Config) GetRunningNumberBash() (lineCount uint8) {
	fullCommand := fmt.Sprintf("%s | %s | sort -u | %s", c.NatstatBash, c.Awk7Bash, c.NameBash)
	log.Printf("执行命令: %s", fullCommand)
	cmd := exec.Command("sh", "-c", fullCommand)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("获取标准输出管道失败: %v", err)
		return
	}
	defer stdout.Close()
	if err := cmd.Start(); err != nil {
		log.Fatalf("启动命令失败: %v", err)
		return
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		lineCount++
		log.Println(scanner.Text()) // 这里也可以顺便打印每一行的内容
	}
	log.Printf("当前节点运行的进程数量: %d", lineCount)
	if err := cmd.Wait(); err != nil {
		// 检查是否是 "exit status 1" (通常意味着 grep 没找到内容)
		if exitError, ok := err.(*exec.ExitError); ok {
			// 如果是 exit status 1，对于 grep 来说通常是“没找到”，不算严重错误
			// 你可以选择只打印警告，或者直接忽略，继续返回 lineCount (此时为 0)
			// log.Printf("命令执行结束，退出码: %d (可能是未找到匹配项)", exitError.ExitCode())

			// 如果退出码不是 1，或者是其他严重错误，再 Fatal
			if exitError.ExitCode() != 1 {
				log.Fatalf("等待命令失败: %v", err)
				return
			}
		} else {
			// 其他类型的错误（如无法启动）
			log.Fatalf("等待命令失败: %v", err)
			return
		}
	}
	return lineCount
}
