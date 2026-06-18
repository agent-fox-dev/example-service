# example-service

A step-by-step example of using **af** (agent-fox) and **spec** to go from a product idea to a working API service. `spec` turns a rough PRD into a structured specification; `af` implements it task-by-task.

The result is a Go HTTP service that ingests events via `POST /v1/events`, stores them in SQLite, and includes auth, health checks, and structured logging.

## 1. Initialize

Start with a `prd.md` describing what you want to build, then initialize the repo:

```shell
af init
```

## 2. Create a spec

Create a specification package from the PRD:

```shell
spec new --name "basic_svc" prd.md
```

## 3. Refine the spec

Run `spec refine` to get feedback and questions about your PRD:

```shell
spec refine 01_basic_svc
```

This produces an assessment and a set of questions that need answers before the spec is complete. Running without `--answers` outputs a template JSON with suggested options:

```json
{
  "questions": [
    {
      "id": "q1",
      "text": "Can you provide a one-sentence Intent statement for this spec?",
      "options": [
        "Use the suggested statement above as-is",
        "Use a modified version",
        "Provide a custom intent statement"
      ],
      "required": true
    }
  ]
}
```

Submit your answers:

```shell
spec refine --answers answers.json 01_basic_svc
```

Where `answers.json` maps question IDs to your choices:

```json
{
  "answers": {
    "q1": "Use the suggested statement above as-is",
    "q2": "The service successfully accepts a POST /events request with a valid bearer token and stores the raw event body in SQLite, returning a 2xx response.",
    "q3": "POST /v1/events",
    "q4": "201 Created with an empty body"
  }
}
```

Repeat the `spec refine` / `--answers` loop until the PRD is ready.

## 4. Generate artifacts

Once the spec is solid, generate the full artifact set:

```shell
spec generate 01_basic_svc
```

This produces:

| File | Purpose |
|------|---------|
| `prd.md` | Finalized product requirements |
| `requirements.json` | Structured requirements |
| `test_spec.json` | Test definitions |
| `tasks.json` | Implementation task graph |

Commit everything before moving on.

## 5. Plan

Create an execution plan from the spec:

```shell
af plan --spec 01_basic_svc
```

`af` analyzes the spec and builds a task graph with dependencies, review gates, and a verification step:

```
Execution Plan
========================================
Specs:         01_basic_svc
Total tasks:   11
Review nodes:  3
Dependencies:  13

Execution order:
  1. drift-review          — Reviewer
  2. pre-review            — Reviewer
  3. Write Tests
  4. Init Go Module
  5. Configuration Loading
  6. Structured Logging
  7. Database Schema
  8. Auth Middleware
  9. POST /v1/events Handler
  10. Health Check Endpoints
  11. Graceful Shutdown
  12. Integration Checkpoint
  13. End-to-End Smoke Tests
  14. Verifier Check
```

Make sure everything is committed and merged into `develop`.

## 6. Code

Start the implementation:

```shell
af code
```

`af` works through the task graph, implementing each task in order:

```
  /\_/\   _
  / o.o \/\ \
 ( > ^ < ) ) )
  \_^/\_/--'
agent-fox v4.0.0-rc1

✔ drift-review     [reviewer]  done (1m 19s)
✔ pre-review       [reviewer]  done (2m 52s)
✔ Write Tests      [coder]     done (12m 13s)
✔ Init Go Module   [coder]     done (2m 56s)
✔ Logging          [coder]     done (5m 5s)
✔ Database         [coder]     done (5m 21s)
✔ Auth Middleware   [coder]     done (6m 0s)
✔ Events Handler   [coder]     done (3m 48s)
✔ Health Checks    [coder]     done (3m 14s)
✔ Shutdown         [coder]     done (7m 4s)
✔ Integration      [coder]     done (10m 48s)
✔ Smoke Tests      [coder]     done (7m 6s)
✔ Verifier         [verifier]  done (4m 3s)

Tasks:  14/14 done
Status: completed
```

When it finishes, you have a working, tested service ready to run.
