# stock_app — ECS Fargate deploy (cheapest / hackathon demo).
# Set these before running any target:
#   ACCOUNT, REGION, CLUSTER, TASK, EXEC_ROLE, TASK_ROLE, SUBNET

ACCOUNT ?= REPLACE_ACCOUNT
REGION  ?= REPLACE_REGION
CLUSTER ?= demo-cluster
TASK    ?= stock-app
SUBNET  ?= REPLACE_SUBNET
EXEC_ROLE ?= REPLACE_EXEC_ROLE_ARN

IMG := $(ACCOUNT).dkr.ecr.$(REGION).amazonaws.com/stock-app:latest

# --- build / push ---
.PHONY: build
build:
	docker build -t $(IMG) .

.PHONY: push
push:
	docker push $(IMG)

# --- register the task definition (taskRoleArn is read from task-definition.json) ---
.PHONY: register
register:
	aws ecs create-task-definition \
		--family $(TASK) \
		--launch-type FARGATE \
		--network-mode awsvpc \
		--container-definitions file://task-definition.json

# --- run (cheapest: a single task, no ALB) ---
# Public IP + a security group that allows :8080 makes it reachable without an ALB.
# The task role (in the task def) grants cloudwatch:PutMetricData -> metrics go to
# CloudWatch (get_metric for the observation bot). To stop paying: `make stop`.
.PHONY: run
run:
	aws ecs run-task \
		--cluster $(CLUSTER) \
		--task-definition $(TASK) \
		--task-execution-role-arn $(EXEC_ROLE) \
		--launch-type FARGATE \
		--network-configuration awsvpcConfiguration="{\"awsvpcConfiguration\":{\"subnets\":[\"$(SUBNET)\"],\"assignPublicIp\":\"ENABLED\"}}"

.PHONY: stop
stop:
	aws ecs stop-task --cluster $(CLUSTER) --task REPLACE_TASKID

# --- IAM roles for Fargate (run once, before `register`) ---
# execution role: pull image + write logs; task role: cloudwatch:PutMetricData.
.PHONY: iam
iam:
	aws iam create-role --role-name stock-app-execution-role \
		--trust-policy-doc '{"Version":"2012-10-10","Statement":[{"Effect":"Allow","Principal":{"Service":"ecs-tasks.amazonaws.com"},"Action":"AssumeRole"}]}' >/dev/null
	aws iam create-role --role-name stock-app-task-role \
		--trust-policy-doc '{"Version":"2012-10-10","Statement":[{"Effect":"Allow","Principal":{"Service":"ecs-tasks.amazonaws.com"},"Action":"AssumeRole"}]}' >/dev/null
	aws iam put-role-policy --role-name stock-app-execution-role --policy-name execution \
		--policy-document file://iam/execution-role-policy.json
	aws iam put-role-policy --role-name stock-app-task-role --policy-name task \
		--policy-document file://iam/task-role-policy.json
	@echo "Set EXEC_ROLE + the task def's taskRoleArn from:"
	@echo "  aws iam get-role --role-name stock-app-execution-role --query Role.Arn --output text"
	@echo "  aws iam get-role --role-name stock-app-task-role   --query Role.Arn --output text"
