PROJECT_NAME := server
BIN_DIR := bin
PROTO_DIR := api
GO_PACKAGE := $(shell head -1 go.mod | awk '{print $$2}')
KIND_CLUSTER := finance-engine
K8S_DIR := deployments/k8s
K8S_NS := finance-engine

.DEFAULT_GOAL := help
.PHONY: help live run build gen tools migrate-up migrate-down \
	kind-up kind-down kind-build kind-deploy kind-migrate kind-redeploy kind-logs kind-status

help: ## Show this help.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

live: ## Run the app with auto reload (requires air).
	air -c .air.toml

run: build ## Build and run the app.
	./${BIN_DIR}/${PROJECT_NAME}

build: ## Build the server binary.
	go build -o ./${BIN_DIR}/ ./cmd/...

gen: ## Generate Go code from .proto files and sqlc queries.
	protoc -I${PROTO_DIR} ${PROTO_DIR}/v1/models/*.proto \
	  --go_out=. --go_opt=module=${GO_PACKAGE} \
	  --go-grpc_out=. --go-grpc_opt=module=${GO_PACKAGE}
	protoc -I${PROTO_DIR} ${PROTO_DIR}/v1/*.proto \
	  --go_out=. --go_opt=module=${GO_PACKAGE} \
	  --go-grpc_out=. --go-grpc_opt=module=${GO_PACKAGE}
	sqlc generate

tools: ## Install dev tools.
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/air-verse/air@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest

migrate-up: ## Apply DB migrations. Requires POSTGRES_DSN env var.
	goose -dir migrations postgres "$$POSTGRES_DSN" up

migrate-down: ## Roll back last DB migration. Requires POSTGRES_DSN env var.
	goose -dir migrations postgres "$$POSTGRES_DSN" down

# ---------------------------------------------------------------------------
# Local Kubernetes (kind) — a live cluster on your machine instead of bare
# processes. Mirrors how this would run in a real cluster (Deployments,
# Services, a migration Job), just scoped to localhost.
# ---------------------------------------------------------------------------

kind-up: ## Create the local kind cluster (maps host :8443 -> the API service).
	kind create cluster --config deployments/kind-cluster.yaml

kind-down: ## Delete the local kind cluster.
	kind delete cluster --name $(KIND_CLUSTER)

kind-build: ## Build the API and migrate images and load them into kind.
	docker build -f deployments/Dockerfile -t finance-engine:local .
	docker build -f deployments/Dockerfile.migrate -t finance-engine-migrate:local .
	kind load docker-image finance-engine:local --name $(KIND_CLUSTER)
	kind load docker-image finance-engine-migrate:local --name $(KIND_CLUSTER)

kind-migrate: ## Run the migration Job against the in-cluster Postgres.
	kubectl -n $(K8S_NS) delete job migrate --ignore-not-found
	kubectl apply -f $(K8S_DIR)/21-migrate-job.yaml
	kubectl -n $(K8S_NS) wait --for=condition=complete job/migrate --timeout=60s
	kubectl -n $(K8S_NS) logs job/migrate

kind-deploy: kind-build ## Apply all manifests, run migrations, deploy the API.
	kubectl apply -f $(K8S_DIR)/00-namespace.yaml
	kubectl apply -f $(K8S_DIR)/10-postgres.yaml
	kubectl -n $(K8S_NS) rollout status deployment/postgres --timeout=90s
	$(MAKE) kind-migrate
	kubectl apply -f $(K8S_DIR)/30-backend.yaml
	kubectl -n $(K8S_NS) rollout status deployment/finance-engine-api --timeout=60s

kind-redeploy: ## Rebuild the API image and roll the Deployment (after a code change).
	docker build -f deployments/Dockerfile -t finance-engine:local .
	kind load docker-image finance-engine:local --name $(KIND_CLUSTER)
	kubectl -n $(K8S_NS) rollout restart deployment/finance-engine-api
	kubectl -n $(K8S_NS) rollout status deployment/finance-engine-api --timeout=60s

kind-logs: ## Tail the API pod's logs.
	kubectl -n $(K8S_NS) logs -f deployment/finance-engine-api

kind-status: ## Show pods/services in the finance-engine namespace.
	kubectl -n $(K8S_NS) get pods,svc,jobs
