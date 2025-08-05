#!/bin/bash

# 性能监控脚本 - 用于监控智能体接口的并发性能

echo "=== 智能体接口并发性能监控 ==="
echo "开始时间: $(date)"

# 配置参数
BASE_URL="http://localhost:8888"
ENDPOINT="/api/v1/chat/completions"  # 替换为实际的智能体接口
CONCURRENCY=100
DURATION=60

# 检查服务是否运行
echo "检查服务状态..."
if ! curl -s "$BASE_URL/health" > /dev/null; then
    echo "❌ 服务未运行，请先启动服务"
    exit 1
fi
echo "✅ 服务运行正常"

# 使用 wrk 进行压力测试（如果安装了的话）
if command -v wrk &> /dev/null; then
    echo "使用 wrk 进行压力测试..."
    wrk -t12 -c$CONCURRENCY -d${DURATION}s --timeout 30s "$BASE_URL$ENDPOINT"
else
    echo "wrk 未安装，使用 Go 脚本进行测试..."
    cd "$(dirname "$0")"
    go run performance_test.go
fi

# 监控系统资源
echo -e "\n=== 系统资源使用情况 ==="
echo "CPU 使用率:"
top -l 1 | grep "CPU usage" || echo "无法获取CPU信息"

echo -e "\n内存使用情况:"
free -h 2>/dev/null || vm_stat | head -10

echo -e "\n数据库连接数:"
# 检查 MySQL 连接数
docker exec coze-mysql mysql -u coze -pcoze123 -e "SHOW STATUS LIKE 'Threads_connected';" 2>/dev/null || echo "无法连接到MySQL"

echo -e "\n=== 性能优化建议 ==="
echo "1. 如果QPS低于预期，检查以下配置："
echo "   - MySQL连接池参数 (MYSQL_MAX_OPEN_CONNS)"
echo "   - Redis连接池配置"
echo "   - 应用服务器并发设置"
echo ""
echo "2. 监控关键指标："
echo "   - 数据库连接数"
echo "   - 内存使用率"
echo "   - CPU使用率"
echo "   - 网络I/O"
echo ""
echo "3. 如果发现瓶颈："
echo "   - 增加数据库连接池大小"
echo "   - 优化SQL查询"
echo "   - 考虑使用缓存"
echo "   - 增加服务器资源"

echo -e "\n结束时间: $(date)"
