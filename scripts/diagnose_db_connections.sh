#!/bin/bash

# 数据库连接池诊断脚本

echo "=== 数据库连接池诊断工具 ==="
echo "开始时间: $(date)"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# MySQL连接参数
MYSQL_HOST="localhost"
MYSQL_PORT="3306"
MYSQL_USER="coze"
MYSQL_PASSWORD="coze123"
MYSQL_DB="opencoze"

echo -e "\n${BLUE}=== 1. MySQL服务状态检查 ===${NC}"

# 检查MySQL服务是否运行
if docker ps | grep -q coze-mysql; then
    echo -e "${GREEN}✅ MySQL容器运行正常${NC}"
else
    echo -e "${RED}❌ MySQL容器未运行${NC}"
    exit 1
fi

echo -e "\n${BLUE}=== 2. MySQL连接数配置检查 ===${NC}"

# 检查MySQL最大连接数配置
echo "检查MySQL最大连接数配置..."
max_connections=$(docker exec coze-mysql mysql -u $MYSQL_USER -p$MYSQL_PASSWORD -e "SHOW VARIABLES LIKE 'max_connections';" 2>/dev/null | tail -n 1 | awk '{print $2}')

if [ ! -z "$max_connections" ]; then
    echo -e "MySQL最大连接数: ${max_connections}"
    if [ "$max_connections" -lt 200 ]; then
        echo -e "${RED}⚠️  MySQL最大连接数过低，建议增加到500+${NC}"
    else
        echo -e "${GREEN}✅ MySQL最大连接数配置合理${NC}"
    fi
else
    echo -e "${RED}❌ 无法获取MySQL最大连接数配置${NC}"
fi

echo -e "\n${BLUE}=== 3. 当前连接状态分析 ===${NC}"

# 检查当前连接数
echo "检查当前活跃连接数..."
current_connections=$(docker exec coze-mysql mysql -u $MYSQL_USER -p$MYSQL_PASSWORD -e "SHOW STATUS LIKE 'Threads_connected';" 2>/dev/null | tail -n 1 | awk '{print $2}')

if [ ! -z "$current_connections" ]; then
    echo -e "当前活跃连接数: ${current_connections}"
    
    # 计算连接使用率
    if [ ! -z "$max_connections" ] && [ "$max_connections" -gt 0 ]; then
        usage_percent=$((current_connections * 100 / max_connections))
        echo -e "连接使用率: ${usage_percent}%"
        
        if [ "$usage_percent" -gt 80 ]; then
            echo -e "${RED}⚠️  连接使用率过高${NC}"
        elif [ "$usage_percent" -gt 60 ]; then
            echo -e "${YELLOW}⚠️  连接使用率较高${NC}"
        else
            echo -e "${GREEN}✅ 连接使用率正常${NC}"
        fi
    fi
else
    echo -e "${RED}❌ 无法获取当前连接数${NC}"
fi

# 检查连接历史峰值
echo -e "\n检查连接历史峰值..."
max_used_connections=$(docker exec coze-mysql mysql -u $MYSQL_USER -p$MYSQL_PASSWORD -e "SHOW STATUS LIKE 'Max_used_connections';" 2>/dev/null | tail -n 1 | awk '{print $2}')

if [ ! -z "$max_used_connections" ]; then
    echo -e "历史最大连接数: ${max_used_connections}"
else
    echo -e "${YELLOW}无法获取历史最大连接数${NC}"
fi

echo -e "\n${BLUE}=== 4. 连接详细信息 ===${NC}"

# 显示当前所有连接的详细信息
echo "当前活跃连接详情:"
docker exec coze-mysql mysql -u $MYSQL_USER -p$MYSQL_PASSWORD -e "
SELECT 
    ID,
    USER,
    HOST,
    DB,
    COMMAND,
    TIME,
    STATE,
    LEFT(INFO, 50) as QUERY_PREVIEW
FROM INFORMATION_SCHEMA.PROCESSLIST 
WHERE COMMAND != 'Sleep' 
ORDER BY TIME DESC 
LIMIT 10;
" 2>/dev/null || echo "无法获取连接详情"

echo -e "\n${BLUE}=== 5. 应用连接池配置检查 ===${NC}"

# 检查应用的连接池环境变量
echo "检查应用连接池环境变量配置..."
if [ -f "/Users/bytedance/Development/workspace/coze-studio/docker/.env.debug" ]; then
    echo "MySQL连接池配置:"
    grep -E "MYSQL_MAX_OPEN_CONNS|MYSQL_MAX_IDLE_CONNS|MYSQL_CONN_MAX_LIFETIME|MYSQL_CONN_MAX_IDLE_TIME" /Users/bytedance/Development/workspace/coze-studio/docker/.env.debug || echo "未找到连接池配置"
else
    echo -e "${RED}❌ 环境配置文件不存在${NC}"
fi

# 检查应用是否正在运行
echo -e "\n检查coze-server应用状态..."
if docker ps | grep -q coze-server; then
    echo -e "${GREEN}✅ coze-server运行正常${NC}"
    
    # 检查应用日志中的连接池相关信息
    echo -e "\n应用启动日志中的连接池信息:"
    docker logs coze-server 2>&1 | grep -i -E "mysql|connection|pool" | tail -5 || echo "未找到相关日志"
else
    echo -e "${RED}❌ coze-server未运行${NC}"
fi

echo -e "\n${BLUE}=== 6. 慢查询检查 ===${NC}"

# 检查是否有慢查询
echo "检查慢查询情况..."
slow_queries=$(docker exec coze-mysql mysql -u $MYSQL_USER -p$MYSQL_PASSWORD -e "SHOW STATUS LIKE 'Slow_queries';" 2>/dev/null | tail -n 1 | awk '{print $2}')

if [ ! -z "$slow_queries" ]; then
    echo -e "慢查询数量: ${slow_queries}"
    if [ "$slow_queries" -gt 0 ]; then
        echo -e "${YELLOW}⚠️  存在慢查询，可能导致连接占用时间过长${NC}"
    else
        echo -e "${GREEN}✅ 无慢查询${NC}"
    fi
fi

echo -e "\n${BLUE}=== 7. 诊断建议 ===${NC}"

echo "基于检查结果的建议:"
echo ""
echo "1. 如果MySQL最大连接数过低:"
echo "   - 在MySQL配置中增加max_connections参数"
echo "   - 重启MySQL服务"
echo ""
echo "2. 如果应用连接池配置未生效:"
echo "   - 检查环境变量是否正确加载"
echo "   - 重启coze-server应用"
echo "   - 验证连接池参数是否被正确应用"
echo ""
echo "3. 如果存在连接泄漏:"
echo "   - 检查业务代码是否正确关闭数据库连接"
echo "   - 查看长时间运行的查询"
echo "   - 优化数据库事务处理逻辑"
echo ""
echo "4. 如果存在慢查询:"
echo "   - 优化慢查询SQL"
echo "   - 添加必要的数据库索引"
echo "   - 考虑查询结果缓存"

echo -e "\n${GREEN}=== 诊断完成 ===${NC}"
echo "结束时间: $(date)"
echo ""
echo "如需进一步分析，请执行:"
echo "- 查看应用详细日志: docker logs coze-server -f"
echo "- 监控实时连接数: watch 'docker exec coze-mysql mysql -u coze -pcoze123 -e \"SHOW STATUS LIKE \\\"Threads_connected\\\";\"'"
echo "- 查看慢查询日志: docker exec coze-mysql mysql -u coze -pcoze123 -e \"SHOW FULL PROCESSLIST;\""
