#!/bin/bash

# KubeMin-Cli 分布式模式启动脚本

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting KubeMin-Cli in Distributed Mode${NC}"

# 检查Redis连接
check_redis() {
    echo -n "Checking Redis connection... "
    if redis-cli -h ${REDIS_HOST} -p ${REDIS_PORT} ping > /dev/null 2>&1; then
        echo -e "${GREEN}OK${NC}"
        return 0
    else
        echo -e "${RED}Failed${NC}"
        return 1
    fi
}

# 启动Redis（如果需要）
start_redis() {
    echo -e "${YELLOW}Starting Redis using Docker...${NC}"
    docker run -d \
        --name kubemin-redis \
        -p 6379:6379 \
        redis:7-alpine \
        redis-server --appendonly yes
    
    # 等待Redis启动
    sleep 3
}

# 设置默认值
REDIS_HOST=${REDIS_HOST:-localhost}
REDIS_PORT=${REDIS_PORT:-6379}
REDIS_ADDR="${REDIS_HOST}:${REDIS_PORT}"
MAX_WORKERS=${MAX_WORKERS:-10}
NODE_ID=${NODE_ID:-$(hostname)}
BIND_ADDR=${BIND_ADDR:-:8008}

# 打印配置
echo "Configuration:"
echo "  Redis Address: ${REDIS_ADDR}"
echo "  Max Workers: ${MAX_WORKERS}"
echo "  Node ID: ${NODE_ID}"
echo "  Bind Address: ${BIND_ADDR}"
echo ""

# 检查Redis
if ! check_redis; then
    read -p "Redis is not available. Start Redis container? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        start_redis
        if ! check_redis; then
            echo -e "${RED}Failed to start Redis. Exiting.${NC}"
            exit 1
        fi
    else
        echo -e "${YELLOW}Warning: Running in local mode without Redis${NC}"
        REDIS_ADDR=""
    fi
fi

# 构建应用（如果需要）
if [ ! -f "./kubemin-cli" ]; then
    echo -e "${YELLOW}Building KubeMin-Cli...${NC}"
    go build -o kubemin-cli cmd/main.go
    if [ $? -ne 0 ]; then
        echo -e "${RED}Build failed. Exiting.${NC}"
        exit 1
    fi
fi

# 设置环境变量
export REDIS_ADDR=${REDIS_ADDR}
export MAX_WORKERS=${MAX_WORKERS}
export NODE_ID=${NODE_ID}

# 启动应用
echo -e "${GREEN}Starting KubeMin-Cli...${NC}"
./kubemin-cli \
    --bind-addr=${BIND_ADDR} \
    --max-workers=${MAX_WORKERS} \
    --id=${NODE_ID} \
    --lock-name=apiserver-lock \
    --exit-on-lost-leader=false

# 如果应用退出，清理资源
echo -e "${YELLOW}Application stopped. Cleaning up...${NC}"
