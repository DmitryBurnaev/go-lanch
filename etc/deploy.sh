#!/bin/sh

if [ "${DEPLOY_MODE}" != "CI" ]
  then
    echo "=== reading $(pwd)/.env file ==="
    export $(cat .env | grep -v ^# | grep -v ^EMAIL | xargs)
fi

echo "=== pulling image ${REGISTRY_URL}/go-lunch:last ==="
docker pull ${REGISTRY_URL}/go-lunch:last

echo "=== restarting service ==="
docker-compose down
docker-compose up -d

echo "=== clearing ==="
echo y | docker image prune -a

echo "=== check status ==="
docker-compose ps
