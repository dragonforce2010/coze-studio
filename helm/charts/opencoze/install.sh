#!/bin/bash

set -a; source .env; set +a;

# Check required environment variables
check_required_vars() {
    local required_vars=(
        "MYSQL_HOST"
        "MYSQL_PORT" 
        "MYSQL_USER"
        "MYSQL_PASSWORD"
        "MYSQL_DATABASE"
        "MYSQL_ROOT_PASSWORD"
        # "TOS_ACCESS_KEY"
        # "TOS_SECRET_KEY"
        # "TOS_ENDPOINT"
        # "TOS_REGION"
        "STORAGE_BUCKET"
        "MQ_NAME_SERVER"
        "RMQ_SECRET_KEY"
        "RMQ_ACCESS_KEY"
    )
    
    for var in "${required_vars[@]}"; do
        if [ -z "${!var}" ]; then
            echo "ERROR: Required environment variable $var is not set"
            echo "Please set environment variables or create .env file"
            exit 1
        fi
    done
    
    echo "All required environment variables are set"
}

# Set variables (read from environment variables, use defaults if not set)
MYSQL_HOST=${MYSQL_HOST}
MYSQL_PORT=${MYSQL_PORT:-3306}
MYSQL_USER=${MYSQL_USER}
MYSQL_PASSWORD=${MYSQL_PASSWORD}
MYSQL_DATABASE=${MYSQL_DATABASE}
MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PASSWORD}
TOS_ACCESS_KEY=${TOS_ACCESS_KEY}
TOS_SECRET_KEY=${TOS_SECRET_KEY}
TOS_ENDPOINT=${TOS_ENDPOINT}
TOS_REGION=${TOS_REGION}
STORAGE_BUCKET=${STORAGE_BUCKET}
MQ_NAME_SERVER=${MQ_NAME_SERVER:-""}
RMQ_SECRET_KEY=${RMQ_SECRET_KEY:-""}
RMQ_ACCESS_KEY=${RMQ_ACCESS_KEY:-""}
ES_ADDR=${ES_ADDR:-""}
ES_USERNAME=${ES_USERNAME:-""}
ES_PASSWORD=${ES_PASSWORD:-""}
REDIS_ADDR=${REDIS_ADDR:-""}
REDIS_PASSWORD=${REDIS_PASSWORD:-""}
NAMESPACE=${NAMESPACE:-"opencoze"}


HELM_BASE_FILES_DIR=opencoze/files

# install basic tools
echo "install basic tools"
apt update && apt install -y mysql-client


init_mysql() {
    echo "init mysql"
    # create mysql schema
    if ! mysql -h ${MYSQL_HOST} -uroot -p"${MYSQL_ROOT_PASSWORD}" -e "CREATE DATABASE IF NOT EXISTS ${MYSQL_DATABASE};CREATE USER IF NOT EXISTS '${MYSQL_USER}'@'%' IDENTIFIED BY '${MYSQL_PASSWORD}';GRANT ALL PRIVILEGES ON ${MYSQL_DATABASE}.* TO '${MYSQL_USER}'@'%';FLUSH PRIVILEGES;"; then
        echo "ERROR: Failed to create mysql user/database"
        return 1
    fi
    
    if ! mysql -h ${MYSQL_HOST} -u"${MYSQL_USER}" -p"${MYSQL_PASSWORD}" ${MYSQL_DATABASE} < ${HELM_BASE_FILES_DIR}/mysql/schema.sql; then
        echo "ERROR: Failed to import mysql schema"
        return 1
    fi
    
    echo "init mysql done successfully"
    return 0
}

init_tos() {
    echo "init tos"
    # check tosutil
    if ! which tosutil; then
        if ! wget https://m645b3e1bb36e-mrap.mrap.accesspoint.tos-global.volces.com/linux/amd64/tosutil; then
            echo "ERROR: Failed to download tosutil"
            return 1
        fi
        if ! chmod a+x tosutil && mv tosutil /usr/local/bin/; then
            echo "ERROR: Failed to install tosutil"
            return 1
        fi
    fi
    
    # config tosutil
    if ! tosutil config -i ${TOS_ACCESS_KEY} -k ${TOS_SECRET_KEY} -e ${TOS_ENDPOINT} -re ${TOS_REGION}; then
        echo "ERROR: Failed to config tosutil"
        return 1
    fi

    # check dir opencoze exists
    if [ ! -d opencoze ]; then
        if ! wget -O opencoze.tgz https://skip-jump.tos-cn-beijing.volces.com/opencoze-0.2.2.tgz; then
            echo "ERROR: Failed to download opencoze.tgz"
            return 1
        fi
        
        if ! tar -zxf opencoze.tgz -C .; then
            echo "ERROR: Failed to extract opencoze.tgz"
            return 1
        fi
    fi

    echo "create bucket"
    if ! tosutil mb tos://${STORAGE_BUCKET}; then
        echo "ERROR: Failed to create bucket"
        return 1
    fi
    
    echo "upload default icon and official plugin icon"
    if ! tosutil cp ${HELM_BASE_FILES_DIR}/minio/default_icon tos://${STORAGE_BUCKET} -r; then
        echo "ERROR: Failed to upload default icon"
        return 1
    fi
    
    if ! tosutil cp ${HELM_BASE_FILES_DIR}/minio/official_plugin_icon tos://${STORAGE_BUCKET} -r; then
        echo "ERROR: Failed to upload official plugin icon"
        return 1
    fi
    
    echo "init tos done successfully"
    return 0
}


init_custom_values() {
    echo "render custom_values.yaml"
    # render custom_values.yaml
    cat > custom_values.yaml <<EOF
# Default values for opencoze.
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

mysql:
  enabled: false
  image:
    repository: opencoze-self-vke-cn-beijing.cr.volces.com/iac/mysql
    tag: 8.4.5
  persistence:
    storageClassName: "ebs-ssd"
    size: "50Gi"

redis:
  enabled: false
# -- coze-server configuration
cozeServer:
  enabled: true
  # -- Number of replicas for the coze-server deployment
  replicaCount: 1

  image:
    repository: opencoze-self-vke-cn-beijing.cr.volces.com/iac/opencoze
    # -- Keep the mirror up to date
    pullPolicy: Always
    tag: 'latest'

  service:
    type: LoadBalancer
    ports:
      - name: http
        port: 8888
        targetPort: 8888
      - name: minio-proxy
        port: 8889
        targetPort: 8889

  env:
    LISTEN_ADDR: ":8888"
    SERVER_HOST: "localhost${LISTEN_ADDR}"
    LOG_LEVEL: "debug"
    MAX_REQUEST_BODY_SIZE: "1073741824"
    STORAGE_TYPE: "minio"
    STORAGE_BUCKET: ${STORAGE_BUCKET}
    TOS_ACCESS_KEY: ${TOS_ACCESS_KEY}
    TOS_SECRET_KEY: ${TOS_SECRET_KEY}
    TOS_REGION: ${TOS_REGION}
    TOS_BUCKET_ENDPOINT: ${STORAGE_BUCKET}.tos-${TOS_REGION}.volces.com
    TOS_ENDPOINT: ${TOS_ENDPOINT}
    MINIO_USE_SSL: "false"
    MINIO_ADDRESS: tos-s3-cn-beijing.ivolces.com:80
    MINIO_BUCKET_NAME: ${STORAGE_BUCKET}
    MINIO_REGION: ${TOS_REGION}
    MINIO_ACCESS_KEY_ID: ${TOS_ACCESS_KEY}
    MINIO_SECRET_ACCESS_KEY: ${TOS_SECRET_KEY}
    MINIO_USE_VIRTUAL_HOST: "true"
    COZE_MQ_TYPE: rmq
    RMQ_SECRET_KEY: ${RMQ_SECRET_KEY}
    RMQ_ACCESS_KEY: ${RMQ_ACCESS_KEY}
    MQ_NAME_SERVER: ${MQ_NAME_SERVER}
    ES_VERSION: "v7"
    ES_ADDR: ${ES_ADDR}
    ES_USERNAME: ${ES_USERNAME}
    ES_PASSWORD: ${ES_PASSWORD}
    VECTOR_STORE_TYPE: "milvus"
    FILE_UPLOAD_COMPONENT_TYPE: "storage"
    MYSQL_HOST: ${MYSQL_HOST}
    MYSQL_PORT: ${MYSQL_PORT}
    MYSQL_USER: ${MYSQL_USER}
    MYSQL_PASSWORD: ${MYSQL_PASSWORD}
    MYSQL_DATABASE: ${MYSQL_DATABASE}
    MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD}
    MYSQL_DSN: "${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(${MYSQL_HOST}:${MYSQL_PORT})/${MYSQL_DATABASE}?charset=utf8mb4&parseTime=True&loc=Local"
    REDIS_ADDR: ${REDIS_ADDR}
    REDIS_PASSWORD: ${REDIS_PASSWORD}

rocketmq:
  enabled: false

elasticsearch:
  enabled: false

minio:
  enabled: true
  image:
    repository: opencoze-self-vke-cn-beijing.cr.volces.com/iac/minio
    tag: RELEASE.2025-06-13T11-33-47Z-cpuv1
  accessKey: minioadmin
  secretKey: minioadmin123
  bucket: ${STORAGE_BUCKET}
  persistence:
    storageClassName: "ebs-ssd"
    size: "50Gi"
  resources:
    limits:
      cpu: 8000m
      memory: 16Gi
    requests:
      cpu: 4000m
      memory: 8Gi

etcd:
  enabled: true
  image:
    repository: opencoze-self-vke-cn-beijing.cr.volces.com/iac/etcd
    tag: 3.5
  persistence:
    storageClassName: "ebs-ssd"
    size: "20Gi"

milvus:
  enabled: true
  image:
    repository: opencoze-self-vke-cn-beijing.cr.volces.com/iac/milvus
    tag: v2.5.10
  bucketName: ${STORAGE_BUCKET}
  persistence:
    storageClassName: "ebs-ssd"
    size: "20Gi"
images:
  busybox: opencoze-self-vke-cn-beijing.cr.volces.com/iac/busybox:latest
  curl: opencoze-self-vke-cn-beijing.cr.volces.com/iac/curl:8.12.1
podAnnotations: {}

securityContext:
  {}
  # capabilities:
  #   drop:
  #   - ALL
  # readOnlyRootFilesystem: true
  # runAsNonRoot: true
  # runAsUser: 1000

# This is for setting up a service more information can be found here: https://kubernetes.io/docs/concepts/services-networking/service/
service:
  # This sets the service type more information can be found here: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types
  type: ClusterIP
  # This sets the ports more information can be found here: https://kubernetes.io/docs/concepts/services-networking/service/#field-spec-ports
  port: 80

# This block is for setting up the ingress for more information can be found here: https://kubernetes.io/docs/concepts/services-networking/ingress/
ingress:
  enabled: false
  className: ''
  annotations:
    {}
    # kubernetes.io/ingress.class: nginx
    # kubernetes.io/tls-acme: "true"
  hosts:
    - host: chart-example.local
      paths:
        - path: /
          pathType: ImplementationSpecific
  tls: []
  #  - secretName: chart-example-tls
  #    hosts:
  #      - chart-example.local
EOF
    if [ $? -ne 0 ]; then
        echo "ERROR: Failed to create custom_values.yaml"
        return 1
    fi
    
    echo "init custom_values.yaml done successfully"
    return 0
}

install_helm() {
    echo "install helm"
    # check helm
    if ! which helm; then
        curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
    fi
    
    echo "install opencoze"
    helm install opencoze opencoze --namespace ${NAMESPACE} --create-namespace -f custom_values.yaml
}

# Check environment variables
check_required_vars

# Main execution flow
echo "Starting installation process..."

echo "1. Initializing TOS..."
# if ! init_tos; then
#     echo "ERROR: TOS initialization failed, exiting installation"
#     exit 1
# fi
rm -rf ${HELM_BASE_FILES_DIR}/minio

echo "2. Initializing MySQL..."
if ! init_mysql; then
    echo "ERROR: MySQL initialization failed, exiting installation"
    exit 1
fi

echo "3. Generating custom_values.yaml..."
if ! init_custom_values; then
    echo "ERROR: custom_values.yaml generation failed, exiting installation"
    exit 1
fi

echo "4. Installing Helm chart..."
if ! install_helm; then
    echo "ERROR: Helm chart installation failed, exiting installation"
    exit 1
fi


echo "All initialization steps completed!"