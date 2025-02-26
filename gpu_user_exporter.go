package main

import (
	"bufio"
	"bytes"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// GPUUserExporter 结构体
type GPUUserExporter struct {
	gpuUserMetric *prometheus.Desc
}

// NewGPUUserExporter 创建 Exporter
func NewGPUUserExporter() *GPUUserExporter {
	return &GPUUserExporter{
		gpuUserMetric: prometheus.NewDesc(
			"gpu_users",
			"Current users occupying GPUs",
			[]string{"gpu", "user"},
			nil,
		),
	}
}

// Describe 发送 metric 描述
func (e *GPUUserExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.gpuUserMetric
}

// Collect 采集 GPU 用户数据
func (e *GPUUserExporter) Collect(ch chan<- prometheus.Metric) {
	gpuUsers := getGPUUsers()
	for gpu, users := range gpuUsers {
		for user := range users {
			ch <- prometheus.MustNewConstMetric(
				e.gpuUserMetric,
				prometheus.GaugeValue,
				1,
				gpu, user,
			)
		}
	}
}

// getGPUUsers 获取 GPU 进程和用户信息
func getGPUUsers() map[string]map[string]struct{} {
	result := make(map[string]map[string]struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 执行 nvidia-smi
	cmd := exec.Command("nvidia-smi", "--query-compute-apps=gpu_uuid,pid", "--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		log.Println("Error executing nvidia-smi:", err)
		return result
	}

	// 解析输出
	scanner := bufio.NewScanner(bytes.NewReader(out))
	pidUserMap := make(map[string]string)
	pidSet := make(map[string]struct{})

	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ", ")
		if len(fields) != 2 {
			continue
		}
		gpuUUID, pid := fields[0], strings.TrimSpace(fields[1])

		// 避免重复查询相同的 PID
		if _, exists := pidSet[pid]; exists {
			continue
		}
		pidSet[pid] = struct{}{}

		wg.Add(1)
		go func(gpuUUID, pid string) {
			defer wg.Done()
			user := getProcessUser(pid)
			if user != "" {
				mu.Lock()
				pidUserMap[pid] = user
				mu.Unlock()
			}
		}(gpuUUID, pid)
	}

	// 等待所有 goroutine 完成
	wg.Wait()

	// 重新遍历并生成 GPU -> 用户映射
	scanner = bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ", ")
		if len(fields) != 2 {
			continue
		}
		gpuUUID, pid := fields[0], strings.TrimSpace(fields[1])

		user, exists := pidUserMap[pid]
		if !exists {
			continue
		}

		if _, found := result[gpuUUID]; !found {
			result[gpuUUID] = make(map[string]struct{})
		}
		result[gpuUUID][user] = struct{}{}
	}

	return result
}

// getProcessUser 通过 PID 获取进程的用户
func getProcessUser(pid string) string {
	cmd := exec.Command("ps", "-o", "user=", "-p", pid)
	out, err := cmd.Output()
	if err != nil {
		log.Println("Error executing ps:", err)
		return ""
	}
	return strings.TrimSpace(string(out))
}

func main() {
	exporter := NewGPUUserExporter()
	prometheus.MustRegister(exporter)

	http.Handle("/metrics", promhttp.Handler())
	log.Println("Starting GPU User Exporter on :9102")
	log.Fatal(http.ListenAndServe(":9102", nil))
}
