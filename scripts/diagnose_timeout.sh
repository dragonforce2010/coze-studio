#!/bin/bash

# HTTP连接超时问题诊断脚本

echo "=== HTTP连接超时问题诊断工具 ==="
echo "开始时间: $(date)"

# 配置参数
BACKEND_HOST="101.126.20.41"
BACKEND_PORT="8888"
HEALTH_ENDPOINT="/api/v1/health"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "\n${BLUE}=== 1. 网络连接测试 ===${NC}"

# 测试网络连通性
echo "测试网络连通性..."
if ping -c 3 $BACKEND_HOST > /dev/null 2>&1; then
    echo -e "${GREEN}✅ 网络连通正常${NC}"
else
    echo -e "${RED}❌ 网络连通异常${NC}"
fi

# 测试端口连通性
echo "测试端口连通性..."
if nc -z $BACKEND_HOST $BACKEND_PORT 2>/dev/null; then
    echo -e "${GREEN}✅ 端口 $BACKEND_PORT 可访问${NC}"
else
    echo -e "${RED}❌ 端口 $BACKEND_PORT 不可访问${NC}"
fi

echo -e "\n${BLUE}=== 2. HTTP服务响应测试 ===${NC}"

# 测试HTTP响应时间
echo "测试HTTP响应时间..."
for i in {1..5}; do
    echo -n "第 $i 次测试: "
    
    # 使用curl测试响应时间
    response_time=$(curl -o /dev/null -s -w "%{time_total}" --connect-timeout 10 --max-time 30 "http://$BACKEND_HOST:$BACKEND_PORT$HEALTH_ENDPOINT" 2>/dev/null)
    exit_code=$?
    
    if [ $exit_code -eq 0 ]; then
        echo -e "${GREEN}响应时间: ${response_time}s${NC}"
    elif [ $exit_code -eq 28 ]; then
        echo -e "${RED}超时 (28秒)${NC}"
    elif [ $exit_code -eq 7 ]; then
        echo -e "${RED}连接失败${NC}"
    else
        echo -e "${RED}错误 (退出码: $exit_code)${NC}"
    fi
    
    sleep 1
done

echo -e "\n${BLUE}=== 3. 并发连接测试 ===${NC}"

# 并发连接测试
echo "测试并发连接能力 (10个并发请求)..."
temp_file="/tmp/concurrent_test_$$"

for i in {1..10}; do
    (
        response_time=$(curl -o /dev/null -s -w "%{time_total}" --connect-timeout 5 --max-time 15 "http://$BACKEND_HOST:$BACKEND_PORT$HEALTH_ENDPOINT" 2>/dev/null)
        exit_code=$?
        echo "请求 $i: 退出码=$exit_code, 响应时间=${response_time}s" >> $temp_file
    ) &
done

wait

echo "并发测试结果:"
if [ -f $temp_file ]; then
    cat $temp_file
    rm $temp_file
fi

echo -e "\n${BLUE}=== 4. 服务器资源检查 ===${NC}"

# 检查Docker容器状态
echo "检查Docker容器状态..."
if command -v docker &> /dev/null; then
    echo "coze-server容器状态:"
    docker ps --filter "name=coze-server" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "无法获取容器信息"
    
    echo -e "\ncoze-server资源使用:"
    docker stats coze-server --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}" 2>/dev/null || echo "无法获取资源信息"
    
    echo -e "\n最近的容器日志:"
    docker logs coze-server --tail=10 2>/dev/null || echo "无法获取日志"
else
    echo -e "${YELLOW}Docker命令不可用，跳过容器检查${NC}"
fi

echo -e "\n${BLUE}=== 5. 数据库连接检查 ===${NC}"

# 检查MySQL连接数
echo "检查MySQL连接数..."
if command -v docker &> /dev/null; then
    mysql_connections=$(docker exec coze-mysql mysql -u coze -pcoze123 -e "SHOW STATUS LIKE 'Threads_connected';" 2>/dev/null | tail -n 1 | awk '{print $2}')
    if [ ! -z "$mysql_connections" ]; then
        echo -e "当前MySQL连接数: ${mysql_connections}"
        if [ "$mysql_connections" -gt 80 ]; then
            echo -e "${RED}⚠️  MySQL连接数过高，可能存在连接池问题${NC}"
        else
            echo -e "${GREEN}✅ MySQL连接数正常${NC}"
        fi
    else
        echo -e "${YELLOW}无法获取MySQL连接数${NC}"
    fi
else
    echo -e "${YELLOW}Docker命令不可用，跳过数据库检查${NC}"
fi

echo -e "\n${BLUE}=== 6. 诊断建议 ===${NC}"

echo "基于测试结果的建议:"
echo "1. 如果网络连通性测试失败:"
echo "   - 检查防火墙设置"
echo "   - 确认服务器IP地址正确"
echo ""
echo "2. 如果HTTP响应时间过长 (>5秒):"
echo "   - 应用MySQL连接池优化配置"
echo "   - 重启coze-server服务"
echo "   - 检查数据库查询性能"
echo ""
echo "3. 如果并发测试失败:"
echo "   - 增加HTTP服务器超时配置"
echo "   - 优化应用程序并发处理能力"
echo ""
echo "4. 如果MySQL连接数过高:"
echo "   - 检查连接池配置是否生效"
echo "   - 排查连接泄漏问题"

echo -e "\n${GREEN}=== 诊断完成 ===${NC}"
echo "结束时间: $(date)"
echo ""
echo "如需进一步分析，请查看:"
echo "- 后端服务日志: docker logs coze-server -f"
echo "- MySQL慢查询日志"
echo "- 系统资源监控"
