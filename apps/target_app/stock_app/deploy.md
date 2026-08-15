# Deploying stock_app to ECS Fargate (cheapest / hackathon demo)

This deploys the target app as a **single ECS Fargate task** — no ALB, no service
autoscaling, pay-per-second. That is the cheapest way to run it, and it stops
paying the moment you stop the task.

> These are the files to use (`Dockerfile`, `task-definition.json`, `Makefile`).
> Nothing here is auto-deployed — you run the commands.

## 1. The cheapest config (what we shipped)

| Setting | Value | Why |
|---|---|---|
| Task size | **0.25 vCPU / 512 MiB** (`task-definition.json`) | Smallest that a Go + pgx-pool app reliably fits. The absolute floor is **0.125 vCPU / 256 MiB** — try it only if you see no OOMs. |
| Load balancer | **None** | The task gets a public IP; no ALB/NLB bill. |
| CloudWatch metrics | **ON** (task role grants `cloudwatch:PutMetricData`) | This is an observation-bot project — `get_metric()` reads real CloudWatch metrics. |
| Uptime | **Run only during the demo** | Fargate is pay-per-second; `make stop` (or `stop-task`) → ~$0. This is the biggest lever. |

Rough cost (us-east-1, **0.125 vCPU / 256 MiB**): ~$0.006/hr → ~$4.50 if left
24/7, **~$0.72/day** if you run it only while demoing. 0.25/512 doubles that.

## 2. Prerequisites (run once)

1. **Apply the schema** to your CockroachDB (hand-run — the app does not migrate):
   ```bash
   for f in migrations/0*.sql; do
     cockroach sql --url "$DATABASE_URL" -f "$f"
   done
   ```
   This creates `target_app`, `stock_db`, and `order_db`.
2. **Secrets** (so they aren't in the task def in cleartext). Create a Secrets
   Manager secret `stock-app` with keys `database-url` and `jwt-secret`, then the
   task def's `secrets` entries pick them up:
   ```bash
   openssl rand -base64 32   # -> the JWT_SECRET value
   ```
3. **ECR** repo + **ECS cluster** (a cluster is free; Fargate is the only compute cost).
4. A **public subnet** + a **security group** allowing inbound `:8080` (from your
   IP, or `0.0.0.0/0` for a public demo) — the task needs a public IP to be reached
   without an ALB.

## 3. Deploy

```bash
cd apps/target_app/stock_app
export ACCOUNT=123456789012 REGION=us-east-1 CLUSTER=demo-cluster
# edit task-definition.json: REPLACE_ACCOUNT / REPLACE_REGION / REPLACE_TASK_ROLE_ARN

make iam        # create execution + task roles (once); then set EXEC_ROLE + taskRoleArn
make build      # docker build -t $IMG .
make push       # docker push $IMG
make register   # aws ecs create-task-definition ...
make run        # aws ecs run-task ... (public IP, no ALB)
```

The app listens on `:8080`; `/health` reports DB up/down.

## 4. When the demo is over

```bash
make stop   # aws ecs stop-task ...  -> you stop paying immediately
```

## 5. CloudWatch metrics (for the observation bot's get_metric())

Shipped **on**: the app publishes to CloudWatch via `PutMetricData` every
`METRICS_FLUSH_INTERVAL` (default 60s; set it lower for a live demo).

What the bot sees in CloudWatch (namespace `stock_app`, dimension `Service=stock_app`):
- `Requests` (Count) — total HTTP requests per flush
- `RequestErrors` (Count) — 5xx per flush
- `RequestLatency` (Milliseconds) — average latency per flush
- **CockroachDB, per database** (extra dimension `Database=target_app|stock_db|order_db`, sampled every `DB_SAMPLE_INTERVAL`, default 15s, plus every real `/health` ping):
  - `DatabaseLatency` (Milliseconds) — average ping latency (failed pings excluded)
  - `DatabaseErrors` (Count) — failed pings in the window
  - `DatabaseUp` (0/1) — last known state, kept between windows

Wiring: `make iam` creates the two Fargate roles —
- **task role** (`stock-app-task-role`, `iam/task-role-policy.json`) → `cloudwatch:PutMetricData`; its ARN goes in the task def's `taskRoleArn`.
- **execution role** (`stock-app-execution-role`, `iam/execution-role-policy.json`) → ECR pull + CloudWatch Logs; its ARN goes in `EXEC_ROLE` / `--task-execution-role-arn`.

Logs are already in CloudWatch Logs (`/ecs/stock-app`) via the `awslogs` driver, so `get_log()` works too; `/health` covers `get_health()`. If you ever want metrics **off** (zero IAM, zero metric cost): remove `AWS_REGION` from the task env — metrics then go to the log stream instead.

## 6. Notes / caveats
- **Container-only** — the app is built for long-lived containers (ECS Fargate):
  persistent DB pools plus background workers (metrics flusher, DB sampler), with
  a graceful SIGTERM drain. There is no Lambda mode.
- `JWT_SECRET` in a task env is fine for a hackathon demo; prefer Secrets Manager
  for anything real.
- Migrations are applied by hand, not by the container.
