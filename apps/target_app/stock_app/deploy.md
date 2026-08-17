# Deploying stock_app to ECS Fargate (cheapest / hackathon demo)

This deploys the target app as a **single ECS Fargate task** — no ALB, no service
autoscaling, pay-per-second. That is the cheapest way to run it, and it stops
paying the moment you stop the task.

> **CI/CD:** `.github/workflows/stock-app-deploy.yml` deploys automatically on
> push to `main` (stock_app files) — see "Auto-deploy via GitHub Actions" below.
> The manual path still works (`Dockerfile`, `task-definition.json`, `Makefile`),
> and `make register` now renders the built image, so there is no stale pin.

## Auto-deploy via GitHub Actions (CI/CD)

`.github/workflows/stock-app-deploy.yml` runs the full loop automatically:

- **Triggers:** push to `main` touching `apps/target_app/stock_app/**`, or the
  manual **Run workflow** button (Actions → *stock_app deploy* → *Run workflow*).
- **Does:** `go vet` + `go test` (gate — broken code never ships) → `docker
  build` + push to ECR (tag = **full commit SHA**, immutable) →
  `create-task-definition` with **that** image → stop the running task + run the
  new one → print the public IP and `http://<ip>:8080/docs`.
- Because it renders the image into the task def at register time, the stale-pin
  bug from before can't recur.

### One-time setup (OIDC — no stored AWS keys)

1. Create the OIDC provider + deploy role (account `204936843729`, region
   `ap-southeast-1`):
   ```bash
   # 1a. OIDC provider for GitHub Actions (idempotent)
   aws iam create-open-id-connect-provider \
     --url https://token.actions.githubusercontent.com \
     --client-ids sts.amazonaws.com 2>/dev/null || true

   # 1b. The deploy role GitHub assumes (trust = this repo's OIDC subject)
   aws iam create-role --role-name stock-app-deploy \
     --assume-role-policy-document file://apps/target_app/stock_app/iam/oidc-trust-policy.json

   # 1c. Its permissions (ECR push + ECS deploy + read the task's public IP)
   aws iam put-role-policy --role-name stock-app-deploy --policy-name deploy \
     --policy-document file://apps/target_app/stock_app/iam/deploy-role-policy.json
   ```
2. In the workflow, set `SUBNET` (a public subnet for the Fargate task) and,
   optionally, `SG` (a security group allowing inbound `:8080`; leave empty to use
   the subnet's default SG, which must then allow `:8080`).
3. Push to `main` — the pipeline deploys. The deploy role is least-privilege
   (only ECR push, the ECS register/run/stop/list/describe actions, and
   `ec2:DescribeNetworkInterfaces`); no long-lived keys live in GitHub.

The API is versioned under `/api/v1` and the docs page is at `/docs`.

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
   This creates `target_app`, `stock_db`, and `order_db` (one URL works for all
   the files — the SQL is fully qualified).
2. **Secrets** (so they aren't in the task def in cleartext). Create a Secrets
   Manager secret `stock-app` with keys `database-url`, `stock-database-url`,
   `order-database-url` and `jwt-secret` — **three URLs, one per database** (same
   host/user/password, only the path differs):
   ```json
   {
     "database-url":          "postgresql://USER:PASS@HOST:26257/target_app?sslmode=verify-full&sslrootcert=<CA pem>&sslcert=<client cert pem>&sslkey=<client key pem>",
     "stock-database-url":    "postgresql://USER:PASS@HOST:26257/stock_db?sslmode=verify-full&sslrootcert=<CA pem>&sslcert=<client cert pem>&sslkey=<client key pem>",
     "order-database-url":    "postgresql://USER:PASS@HOST:26257/order_db?sslmode=verify-full&sslrootcert=<CA pem>&sslcert=<client cert pem>&sslkey=<client key pem>",
     "jwt-secret":           "<openssl rand -base64 32>"
   }
   ```
   `DATABASE_URL` must end with `/target_app`, `STOCK_DATABASE_URL` with
   `/stock_db`, `ORDER_DATABASE_URL` with `/order_db` — the app verifies each
   schema at boot and in `/health`, and logs `startup.schema_check_failed` with
   a hint when one points at the wrong database. (Single-database layout: put
   the same URL in all three keys — the tables just have to exist in that one
   database.)
   The task def's `secrets` entries pick the keys up; **a task fails to start if
   a key is missing**, so add all three keys before re-running the task.
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
export ACCOUNT=123456789012 REGION=ap-southeast-1 CLUSTER=demo-cluster SUBNET=subnet-xxxx
make iam        # create the execution + task IAM roles (once)
make login      # docker login to ECR (session creds)
make build      # docker build -t $IMG .   (tag = short git SHA)
make push       # docker push $IMG
make register   # render $IMG into task-definition.json, then create-task-definition
make run        # aws ecs run-task ... (public IP, no ALB)
```

- `make register` renders the image `make build` tagged into
  `task-definition.json` (its `image` field is a placeholder), so the registered
  def runs your build — never a stale pin. Needs `jq`.
- `make run` uses `--task-definition stock-app` (latest revision); the execution
  role is in the task def, so it is no longer passed as a flag.

The app listens on `:8080`; `/health` reports DB up/down, the API is under
`/api/v1`, and the docs are at `/docs`.

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
- `DATABASE_URL` / `STOCK_DATABASE_URL` / `ORDER_DATABASE_URL` and `JWT_SECRET`
  come from the `stock-app` Secrets Manager secret (one key per database — see
  step 2), not cleartext in the task def.
- Migrations are applied by hand, not by the container.
