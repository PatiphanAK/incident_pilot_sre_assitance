.PHONY: help dev test build local invoke deploy clean

help:            ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*##"};{printf "  %-14s %s\n", $$1, $$2}'

dev:            ## Run the local FastAPI shim (no Docker, no AWS account needed)
	uv run fastapi dev local_dev.py

test:           ## Run unit tests (stdlib runtime; no network)
	uv run pytest -q

build:          ## SAM build (needs Docker for the python3.13 build image)
	sam build

local:          ## SAM local API on :3000 (needs Docker); then curl localhost:3000/runs
	sam build && sam local start-api

invoke:         ## SAM local invoke with a sample API GW event (needs Docker)
	sam build && sam local invoke CommandFunction --event events/post_runs.json

deploy:         ## Guided SAM deploy to an AWS account
	sam deploy --guided

clean:          ## Remove SAM build artifacts
	rm -rf .aws-sam
