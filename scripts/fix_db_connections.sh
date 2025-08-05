#!/bin/bash

# 数据库连接池优化修复脚本

echo "=== 数据库连接池优化修复工具 ==="
echo "开始时间: $(date)"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 切换到docker目录
cd "$(dirname "$0")/../docker" || exit 1

echo -e "\n${BLUE}=== 1. 停止相关服务 ===${NC}"
echo "停止coze-server和MySQL服务..."

# 停止服务
if command -v docker &> /dev/null; then
    docker compose -f docker-compose-debug.yml stop coze-server mysql 2>/dev/null || \
    docker-compose -f docker-compose-debug.yml stop coze-server mysql 2>/dev/null || \
    echo -e "${YELLOW}使用docker命令停止服务${NC}"
else
    echo -e "${RED}Docker命令不可用${NC}"
    exit 1
fi

echo -e "\n${BLUE}=== 2. 清理MySQL数据（可选） ===${NC}"
read -p "是否清理MySQL数据以确保配置生效？(y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "清理MySQL数据目录..."
    sudo rm -rf ./data/mysql/* 2>/dev/null || echo "清理完成或目录不存在"
    echo -e "${YELLOW}注意：数据已清理，首次启动将重新初始化数据库${NC}"
fi

echo -e "\n${BLUE}=== 3. 启动优化后的服务 ===${NC}"
echo "启动MySQL服务（使用新的连接配置）..."

# 启动MySQL
if command -v docker &> /dev/null; then
    docker compose -f docker-compose-debug.yml up -d mysql 2>/dev/null || \
    docker-compose -f docker-compose-debug.yml up -d mysql 2>/dev/null
else
    echo -e "${RED}Docker命令不可用${NC}"
    exit 1
fi

echo "等待MySQL服务启动..."
sleep 10

# 检查MySQL是否启动成功
for i in {1..30}; do
    if docker exec coze-mysql mysql -u coze -pcoze123 -e "SELECT 1;" &>/dev/null; then
        echo -e "${GREEN}✅ MySQL服务启动成功${NC}"
        break
    fi
    echo "等待MySQL启动... ($i/30)"
    sleep 2
done

echo -e "\n${BLUE}=== 4. 验证MySQL配置 ===${NC}"
echo "检查MySQL最大连接数配置..."

max_connections=$(docker exec coze-mysql mysql -u coze -pcoze123 -e "SHOW VARIABLES LIKE 'max_connections';" 2>/dev/null | tail -n 1 | awk '{print $2}')

if [ ! -z "$max_connections" ]; then
    echo -e "MySQL最大连接数: ${GREEN}${max_connections}${NC}"
    if [ "$max_connections" -ge 1000 ]; then
        echo -e "${GREEN}✅ MySQL最大连接数配置正确${NC}"
    else
        echo -e "${RED}❌ MySQL最大连接数配置可能未生效${NC}"
    fi
else
    echo -e "${RED}❌ 无法获取MySQL配置${NC}"
fi

echo -e "\n${BLUE}=== 5. 启动应用服务 ===${NC}"
echo "启动coze-server应用..."

# 启动应用服务
docker compose -f docker-compose-debug.yml up -d coze-server 2>/dev/null || \
docker-compose -f docker-compose-debug.yml up -d coze-server 2>/dev/null

echo "等待应用服务启动..."
sleep 5

# 检查应用是否启动成功
for i in {1..20}; do
    if curl -s http://localhost:8888/api/v1/health &>/dev/null; then
        echo -e "${GREEN}✅ coze-server服务启动成功${NC}"
        break
    fi
    echo "等待应用启动... ($i/20)"
    sleep 3
done

echo -e "\n${BLUE}=== 6. 验证连接池配置 ===${NC}"
echo "检查应用日志中的连接池信息..."

# 检查应用日志
docker logs coze-server 2>&1 | grep -i -E "mysql|connection|pool" | tail -10 || echo "未找到相关日志"

echo -e "\n${BLUE}=== 7. 连接测试 ===${NC}"
echo "进行连接测试..."

# 简单的连接测试
for i in {1..5}; do
    echo -n "测试 $i: "
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:8888/api/v1/health | grep -q "200"; then
        echo -e "${GREEN}成功${NC}"
    else
        echo -e "${RED}失败${NC}"
    fi
    sleep 1
done

echo -e "\n${GREEN}=== 优化完成 ===${NC}"
echo "结束时间: $(date)"
echo ""
echo "配置摘要:"
echo "- MySQL最大连接数: 1000"
echo "- 应用连接池最大连接数: 500"
echo "- 应用连接池最大空闲连接数: 100"
echo ""
echo "下一步建议:"
echo "1. 运行数据库连接诊断: ./scripts/diagnose_db_connections.sh"
echo "2. 进行压力测试验证: cd scripts && go run performance_test.go"
echo "3. 监控连接数变化: watch 'docker exec coze-mysql mysql -u coze -pcoze123 -e \"SHOW STATUS LIKE \\\"Threads_connected\\\";\"'"
echo ""
echo "如果仍有问题，请查看:"
echo "- 应用日志: docker logs coze-server -f"
echo "- MySQL日志: docker logs coze-mysql -f"
