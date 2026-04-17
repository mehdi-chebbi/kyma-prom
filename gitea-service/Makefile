.PHONY: help build test run docker-build k8s-deploy k8s-delete k8s-logs k8s-status clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the Go binary
	@echo "Building gitea-service..."
	go build -o bin/gitea-service ./cmd/server

test: ## Run tests
	@echo "Running tests..."
	go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

run: ## Run the service locally
	@echo "Running gitea-service..."
	go run ./cmd/server/main.go

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t gitea-service:latest .

docker-run: ## Run Docker container locally
	@echo "Running Docker container..."
	docker run --rm -p 8081:8081 -p 9091:9091 \
		-e GITEA_URL=http://host.docker.internal:3000 \
		-e GITEA_TOKEN=change-me \
		-e LDAP_MANAGER_URL=http://host.docker.internal:8080 \
		-e JWT_SECRET=change-me \
		gitea-service:latest

k8s-deploy: docker-build ## Build and deploy to Kubernetes
	@echo "Deploying to Kubernetes..."
	kubectl apply -f k8s/
	@echo "Waiting for rollout..."
	kubectl rollout status deployment/gitea-service -n dev-platform

k8s-delete: ## Delete Kubernetes resources
	@echo "Deleting Kubernetes resources..."
	kubectl delete -f k8s/ --ignore-not-found=true

k8s-restart: ## Restart Kubernetes deployment
	@echo "Restarting deployment..."
	kubectl rollout restart deployment/gitea-service -n dev-platform
	kubectl rollout status deployment/gitea-service -n dev-platform

k8s-logs: ## Show Kubernetes logs
	@echo "Showing logs..."
	kubectl logs -f -l app=gitea-service -n dev-platform --tail=50

k8s-logs-all: ## Show all Kubernetes logs
	@echo "Showing all logs..."
	kubectl logs -l app=gitea-service -n dev-platform --all-containers=true

k8s-status: ## Show Kubernetes deployment status
	@echo "=== Pods ==="
	kubectl get pods -n dev-platform -l app=gitea-service
	@echo ""
	@echo "=== Services ==="
	kubectl get svc -n dev-platform -l app=gitea-service
	@echo ""
	@echo "=== Ingress ==="
	kubectl get ingressroute gitea-service -n dev-platform
	@echo ""
	@echo "=== HPA ==="
	kubectl get hpa gitea-service -n dev-platform
	@echo ""
	@echo "=== Events ==="
	kubectl get events -n dev-platform --field-selector involvedObject.name=gitea-service --sort-by='.lastTimestamp' | tail -10

k8s-describe: ## Describe Kubernetes deployment
	kubectl describe deployment gitea-service -n dev-platform

k8s-port-forward: ## Port forward to service
	@echo "Port forwarding to service..."
	kubectl port-forward -n dev-platform svc/gitea-service 8081:80

k8s-metrics-forward: ## Port forward to metrics
	@echo "Port forwarding to metrics..."
	kubectl port-forward -n dev-platform svc/gitea-service 9091:9091

k8s-exec: ## Exec into pod
	kubectl exec -it deployment/gitea-service -n dev-platform -- sh

k8s-top: ## Show resource usage
	kubectl top pod -l app=gitea-service -n dev-platform

health-check: ## Check service health
	@echo "Checking health..."
	@curl -s http://localhost:8081/health | jq

ready-check: ## Check service readiness
	@echo "Checking readiness..."
	@curl -s http://localhost:8081/ready | jq

metrics: ## Show Prometheus metrics
	@echo "Fetching metrics..."
	@curl -s http://localhost:9091/metrics | grep gitea_

graphql-health: ## Test GraphQL health query
	@echo "Testing GraphQL health query..."
	@curl -s -X POST http://localhost:8081/graphql \
		-H "Content-Type: application/json" \
		-d '{"query":"query { health { status gitea ldapManager timestamp } }"}' | jq

test-auth: ## Test authentication flow (requires TOKEN env var)
	@if [ -z "$(TOKEN)" ]; then \
		echo "Error: TOKEN environment variable not set"; \
		echo "Usage: make test-auth TOKEN=your-jwt-token"; \
		exit 1; \
	fi
	@echo "Testing authenticated query..."
	@curl -s -X POST http://localhost:8081/graphql \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $(TOKEN)" \
		-d '{"query":"query { myRepositories { name fullName } }"}' | jq

test-repos: ## Test repository query (requires TOKEN env var)
	@if [ -z "$(TOKEN)" ]; then \
		echo "Error: TOKEN environment variable not set"; \
		echo "Usage: make test-repos TOKEN=your-jwt-token"; \
		exit 1; \
	fi
	@echo "Testing repository query..."
	@curl -s -X POST http://localhost:8081/graphql \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $(TOKEN)" \
		-d '{"query":"query { myRepositories { id name fullName description private htmlUrl cloneUrl } }"}' | jq

test-stats: ## Test repository stats (requires TOKEN env var)
	@if [ -z "$(TOKEN)" ]; then \
		echo "Error: TOKEN environment variable not set"; \
		echo "Usage: make test-stats TOKEN=your-jwt-token"; \
		exit 1; \
	fi
	@echo "Testing repository stats..."
	@curl -s -X POST http://localhost:8081/graphql \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $(TOKEN)" \
		-d '{"query":"query { repositoryStats { totalRepositories privateRepositories publicRepositories totalStars totalForks } }"}' | jq

lint: ## Run Go linter
	@echo "Running linter..."
	golangci-lint run ./...

fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

deps: ## Download Go dependencies
	@echo "Downloading dependencies..."
	go mod download

tidy: ## Tidy Go dependencies
	@echo "Tidying dependencies..."
	go mod tidy

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

setup-dev: ## Setup development environment
	@echo "Setting up development environment..."
	@echo "Installing dependencies..."
	go mod download
	@echo "Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Installing jq (if not present)..."
	@which jq > /dev/null || echo "Please install jq: https://stedolan.github.io/jq/"
	@echo "Setup complete!"

verify-secrets: ## Verify JWT secrets match
	@echo "Verifying JWT secrets..."
	@echo "LDAP Manager JWT:"
	@kubectl get secret ldap-manager-secret -n dev-platform -o jsonpath='{.data.JWT_SECRET}' | base64 -d
	@echo ""
	@echo "Gitea Service JWT:"
	@kubectl get secret gitea-service-secret -n dev-platform -o jsonpath='{.data.JWT_SECRET}' | base64 -d
	@echo ""

get-token: ## Get JWT token from LDAP Manager (requires UID and PASSWORD env vars)
	@if [ -z "$(UID)" ] || [ -z "$(PASSWORD)" ]; then \
		echo "Error: UID and PASSWORD environment variables not set"; \
		echo "Usage: make get-token UID=john.doe PASSWORD=password123"; \
		exit 1; \
	fi
	@echo "Getting JWT token..."
	@curl -s -X POST http://ldap-manager.localhost/graphql \
		-H "Content-Type: application/json" \
		-d '{"query":"mutation { login(uid: \"$(UID)\", password: \"$(PASSWORD)\") { token } }"}' | jq -r '.data.login.token'

full-test: ## Run full test flow
	@echo "Running full test flow..."
	@echo ""
	@echo "1. Getting JWT token..."
	@TOKEN=$$(curl -s -X POST http://ldap-manager.localhost/graphql \
		-H "Content-Type: application/json" \
		-d '{"query":"mutation { login(uid: \"john.doe\", password: \"password123\") { token } }"}' | \
		jq -r '.data.login.token'); \
	if [ "$$TOKEN" = "null" ] || [ -z "$$TOKEN" ]; then \
		echo "Failed to get token. Is LDAP Manager running?"; \
		exit 1; \
	fi; \
	echo "Got token: $$TOKEN"; \
	echo ""; \
	echo "2. Testing health check..."; \
	curl -s http://localhost:8081/health | jq; \
	echo ""; \
	echo "3. Testing my repositories..."; \
	curl -s -X POST http://localhost:8081/graphql \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $$TOKEN" \
		-d '{"query":"query { myRepositories { name fullName } }"}' | jq; \
	echo ""; \
	echo "4. Testing repository stats..."; \
	curl -s -X POST http://localhost:8081/graphql \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $$TOKEN" \
		-d '{"query":"query { repositoryStats { totalRepositories } }"}' | jq; \
	echo ""; \
	echo "Full test complete!"

all: clean deps build test docker-build ## Clean, build, test, and create Docker image
