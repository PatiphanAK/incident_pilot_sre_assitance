"""Incident Pilot — command-submission Lambda.

A thin, DB-free command layer: receive an incident command, validate it,
generate a ``run_id``, enqueue a job to SQS, and return ``202`` in seconds.
The 5–20 min agent loop runs later on the ECS Fargate worker (``backend_run``).

The package is laid out flat (no domain/application/infrastructure subfolders)
because the layer is ~8 files; the hexagonal intent is preserved by import
rules:

- :mod:`command_lambda.domain`    — models, ports (Protocol), errors. No AWS/IO.
- :mod:`command_lambda.validate`  — application use case (validation). stdlib only.
- :mod:`command_lambda.submit`    — application use case (submission). stdlib only.
- :mod:`command_lambda.sqs_publisher`, :mod:`command_lambda.run_id`,
  :mod:`command_lambda.apigw`, :mod:`command_lambda.handler` — adapters.
"""

__all__ = ["__version__"]

__version__ = "0.1.0"
