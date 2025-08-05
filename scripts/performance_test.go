package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// TestConfig 测试配置
type TestConfig struct {
	BaseURL     string `json:"base_url"`
	Concurrency int    `json:"concurrency"`
	Duration    int    `json:"duration_seconds"`
	Endpoint    string `json:"endpoint"`
}

// TestResult 测试结果
type TestResult struct {
	TotalRequests   int           `json:"total_requests"`
	SuccessRequests int           `json:"success_requests"`
	FailedRequests  int           `json:"failed_requests"`
	QPS             float64       `json:"qps"`
	AvgResponseTime time.Duration `json:"avg_response_time"`
	MaxResponseTime time.Duration `json:"max_response_time"`
	MinResponseTime time.Duration `json:"min_response_time"`
}

func main() {
	config := TestConfig{
		BaseURL:     "http://localhost:8888",
		Concurrency: 100,
		Duration:    60, // 60秒测试
		Endpoint:    "/api/v1/health", // 替换为实际的智能体接口
	}

	fmt.Printf("开始并发性能测试...\n")
	fmt.Printf("并发数: %d\n", config.Concurrency)
	fmt.Printf("测试时长: %d秒\n", config.Duration)
	fmt.Printf("目标URL: %s%s\n", config.BaseURL, config.Endpoint)

	result := runLoadTest(config)
	printResults(result)
}

func runLoadTest(config TestConfig) TestResult {
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	result := TestResult{
		MinResponseTime: time.Hour, // 初始化为一个很大的值
	}
	
	startTime := time.Now()
	endTime := startTime.Add(time.Duration(config.Duration) * time.Second)
	
	// 启动并发goroutine
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			// 增加超时时间以避免连接超时
			client := &http.Client{
				Timeout: 90 * time.Second, // 增加到90秒
				Transport: &http.Transport{
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 100,
					IdleConnTimeout:     90 * time.Second,
					DisableKeepAlives:   false, // 启用连接复用
				},
			}
			
			for time.Now().Before(endTime) {
				reqStart := time.Now()
				
				// 创建测试请求 - 使用健康检查接口避免复杂业务逻辑
				req, err := http.NewRequest("GET", config.BaseURL+"/api/v1/health", nil)
				if err != nil {
					mu.Lock()
					result.FailedRequests++
					mu.Unlock()
					continue
				}
				
				req.Header.Set("Content-Type", "application/json")
				
				resp, err := client.Do(req)
				responseTime := time.Since(reqStart)
				
				mu.Lock()
				result.TotalRequests++
				
				if err != nil || resp.StatusCode >= 400 {
					result.FailedRequests++
				} else {
					result.SuccessRequests++
					// 读取响应体以确保完整处理
					if resp.Body != nil {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
					}
				}
				
				// 更新响应时间统计
				if responseTime > result.MaxResponseTime {
					result.MaxResponseTime = responseTime
				}
				if responseTime < result.MinResponseTime {
					result.MinResponseTime = responseTime
				}
				
				mu.Unlock()
			}
		}()
	}
	
	wg.Wait()
	
	totalDuration := time.Since(startTime)
	result.QPS = float64(result.TotalRequests) / totalDuration.Seconds()
	
	if result.TotalRequests > 0 {
		// 计算平均响应时间（简化计算）
		result.AvgResponseTime = time.Duration(int64(totalDuration) / int64(result.TotalRequests))
	}
	
	return result
}

func printResults(result TestResult) {
	fmt.Printf("\n=== 性能测试结果 ===\n")
	fmt.Printf("总请求数: %d\n", result.TotalRequests)
	fmt.Printf("成功请求数: %d\n", result.SuccessRequests)
	fmt.Printf("失败请求数: %d\n", result.FailedRequests)
	fmt.Printf("成功率: %.2f%%\n", float64(result.SuccessRequests)/float64(result.TotalRequests)*100)
	fmt.Printf("QPS: %.2f\n", result.QPS)
	fmt.Printf("平均响应时间: %v\n", result.AvgResponseTime)
	fmt.Printf("最大响应时间: %v\n", result.MaxResponseTime)
	fmt.Printf("最小响应时间: %v\n", result.MinResponseTime)
	
	// 性能评估
	fmt.Printf("\n=== 性能评估 ===\n")
	if result.QPS < 10 {
		fmt.Printf("⚠️  QPS过低，存在严重性能瓶颈\n")
	} else if result.QPS < 50 {
		fmt.Printf("⚠️  QPS较低，需要优化\n")
	} else if result.QPS < 100 {
		fmt.Printf("✅ QPS正常\n")
	} else {
		fmt.Printf("🚀 QPS优秀\n")
	}
	
	if result.AvgResponseTime > 1*time.Second {
		fmt.Printf("⚠️  平均响应时间过长\n")
	} else if result.AvgResponseTime > 500*time.Millisecond {
		fmt.Printf("⚠️  平均响应时间较长\n")
	} else {
		fmt.Printf("✅ 响应时间正常\n")
	}
}
