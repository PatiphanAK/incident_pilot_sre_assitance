# stock_app — ECS Fargate deploy (cheapest / hackathon demo).
#
# Run from the stock_app dir, OR from the repo root:
#     make -C apps/target_app/stock_app <target>
# Paths and the Docker build context resolve to this Makefile's directory,
# so the targets work no matter where `make` is invoked from.
#
# Before running a deploy target, set these (via export, or `make VAR=value`):
#     ACCOUNT  REGION  CLUSTER  SUBNET  EXEC_ROLE  TASK_ROLE
# The defaults below are REPLACE_* placeholders — no real values are hard-coded.

# Directory that holds this Makefile, so file refs + build context are
# correct regardless of the caller's cwd.
MAKEFILE_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))

ACCOUNT   ?= REPLACE_ACCOUNT
REGION    ?= REPLACE_REGION
CLUSTER   ?= demo-cluster
TASK      ?= stock-app
SUBNET    ?= REPLACE_SUBNET
EXEC_ROLE ?= REPLACE_EXEC_ROLE_ARN
TASK_ROLE ?= REPLACE_TASK_ROLE_ARN
TASK_ID   ?=

# Immutable image tag = the git commit SHA (never `:latest`).
GIT_SHA := $(shell git -C $(MAKEFILE_DIR) rev-parse --short HEAD)
IMG     := $(ACCOUNT).dkr.ecr.$(REGION).amazonaws.com/stock-app:$(GIT_SHA)

.PHONY: build login push register run stop iam

# --- build (context is this dir, so Dockerfile + .dockerignore are found) ---
.PHONY: build
build:
	docker build -t $(IMG) $(MAKEFILE_DIR)

# --- ECR login (uses the session creds; no hard-coded secret) ---
.PHONY: login
login:
	aws ecr get-login-password --region $(REGION) | \
		docker login --password-stdin $(ACCOUNT).dkr.ecr.$(REGION).amazonaws.com

# --- push the tagged image ---
.PHONY: push
push:
	docker push $(IMG)

# --- register the task definition (ARNs + secret refs live in task-definition.json) ---
.PHONY: register
register:
	aws ecs create-task-definition \
		--family $(TASK) \
		--network-mode awsvpc \
		--container-definitions file://$(MAKEFILE_DIR)/task-definition.json

# --- run (cheapest: a single task, no ALB) ---
# A public IP + a security group that allows :8080 makes it reachable without an ALB.
# The task role (in the task def) grants cloudwatch:PutMetricData -> metrics go to
# CloudWatch (get_metric for the observation bot). To stop paying: `make stop TASK_ID=...`.
.PHONY: run
run:
	aws ecs run-task \
		--cluster $(CLUSTER) \
		--task-definition $(TASK) \
		--task-execution-role-arn $(EXEC_ROLE) \
		--launch-type FARGATE \
		--network-configuration "awsvpcConfiguration={\"awsvpcConfiguration\":{\"subnets\":[\"$(SUBNET)\"],\"assignPublicIp\":\"ENABLED\"}}"

# --- stop a task (MANUAL; requires TASK_ID, so it can never fire by accident) ---
.PHONY: stop
stop:
	@test "$(TASK_ID)" != "" || { echo "TASK_ID is required: make stop TASK_ID=<task-arn>"; exit 1; }
	aws ecs stop-task --cluster $(CLUSTER) --task $(TASK_ID)

# --- IAM roles for Fargate (run once, before `register`) ---
# execution role: ECR pull + CloudWatch Logs + read its OWN Secrets Manager secret
# task role:     cloudwatch:PutMetricData
.PHONY: iam
iam:
	aws iam create-role --role-name stock-app-execution-role \
		--trust-policy-doc '{"Version":"2012-10-10","Statement":[{"Effect":"Allow","Principal":{"Service":"ecs-tasks.amazonaws.com"},"Action":"AssumeRole"}]}' >/dev/null
	aws iam create-role --role-name stock-app-task-role \
		--trust-policy-doc '{"Version":"2012-10-10","Statement":[{"Effect":"Allow","Principal":{"Service":"ecs-tasks.amazonaws.com"},"Action":"AssumeRole"}]}' >/dev/null
	aws iam put-role-policy --role-name stock-app-execution-role --policy-name execution \
		--policy-document file://$(MAKEFILE_DIR)/iam/execution-role-policy.json
	aws iam put-role-policy --role-name stock-app-execution-role --policy-name secrets \
		--policy-document file://$(MAKEFILE_DIR)/iam/execution-secrets-policy.json
	aws iam put-role-policy --role-name stock-app-task-role --policy-name task \
		--policy-document file://$(MAKEFILE_DIR)/iam/task-role-policy.json
	@echo "Set EXEC_ROLE + the task def's taskRoleArn from:"
	@echo "  aws iam get-role --role-name stock-app-execution-role --query Role.Arn --output text"
	@echo "  aws iam get-role --role-name stock-app-task-role   --query Role.Arn --output text"
