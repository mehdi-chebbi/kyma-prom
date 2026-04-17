#!/bin/bash

echo "Loading environment variables..."
export LDAP_URL=ldap://localhost:30000
export LDAP_BASE_DN=dc=devplatform,dc=local
export LDAP_BIND_DN=cn=admin,dc=devplatform,dc=local
export LDAP_BIND_PASSWORD=admin123
export JWT_SECRET=dev-secret-key-change-in-production-12345678
export PORT=8080
export METRICS_PORT=9090
export ENVIRONMENT=development
export LOG_LEVEL=debug
export LDAP_POOL_SIZE=5
export STARTING_UID=10000
export STARTING_GID=10000

echo "Starting LDAP Manager Service..."
echo ""
echo "Service will be available at:"
echo "  - GraphQL API: http://localhost:8080/graphql"
echo "  - Health Check: http://localhost:8080/health"
echo "  - Readiness: http://localhost:8080/ready"
echo "  - Metrics: http://localhost:9090/metrics"
echo ""

cd "$(dirname "$0")"
go run cmd/server/main.go
