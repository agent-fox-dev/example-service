# example-service
Example how to use af and spec to build a simple API service

## Project setup

Prepare a `prd.md` file.

Initialize the repo to work with `agent-fox`:

```shell
af init
```

## Spec creation and refinement

Create the first specification package from the `prd.md`:

```shell
spec new --name "basic_svc" prd.md
```

Let `spec refine` do a first pass on the prd.md and provide feedback:

```shell
spec refine 01_basic_svc
```

`spec refine` creates an assessment of the prd and creates a list of questions that must be answered to finalize the PRD.

Submit your answers:

```shell
spec refine --answers answers.json 01_basic_svc
```

Example of an `answers.json` file:

```json
{
    "answers": {
    "q1": "Use the suggested statement above as-is",
    "q2": "The service successfully accepts a POST /events request with a valid bearer token and stores the raw event body in SQLite, returning a 2xx response.",
    "q3": "Running the same sequence of af and spec commands based on the same prd.md file will allow us to evaluate the performance of af and spec commands.",
    "q4": "POST /v1/events",
    "q5": "201 Created with an empty body",
    "q6": "Require Content-Type: application/json and a non-empty body; reject with 400 otherwise",
    "q7": "Single table with columns: id (UUID), payload (TEXT/JSON), received_at (DATETIME, auto-set on insert)",
    "q8": "The agent-fox team"
  }
}
```

Running `spec refine` again, without providing answers to pending questions issues a template .json file with suggestions:

```json
{
  "questions": [
    {
      "id": "q1",
      "text": "Can you provide a one-sentence Intent statement for this spec? For example: 'To provide a minimal, verifiable HTTP ingestion service that receives and persists audit events from agent-fox instances for integration testing purposes.'",
      "context": "The Intent section is a required field in the spec format and must be a discrete, clearly articulated statement rather than prose embedded in an Overview.",
      "options": [
        "Use the suggested statement above as-is",
        "Use a modified version of the suggested statement",
        "Provide a custom intent statement"
      ],
      "required": true
    },
    {
      "id": "q2",
      "text": "Even for an early-phase test service, can you define at least one minimal, verifiable success criterion for the Goals section?",
      "context": "Without any goals, there is no basis for determining when the service is 'done' or passing. Even a simple functional goal helps downstream artifact generation.",
      "options": [
        "The service successfully accepts a POST /events request with a valid bearer token and stores the raw event body in SQLite, returning a 2xx response.",
        "The service passes a defined end-to-end smoke test: send one event, confirm it is persisted in the database.",
        "No goals \u2014 this is a throwaway test harness and correctness is judged informally."
      ],
      "required": true
    }
  ]
}
```

Based on the provided answers, the PRD will be updated and `spec` will ask further questions should the PRD still not be ready for implementation. 
You simply repeat the above `spec refine` -> `spec refine --answers` loop until your are satisfied with the quality of the PRD.

Once the PRD is "good enough", let `spec` create the remaining artefacts:

```shell
spec generate 01_basic_svc
```

The folder should now contain the following files:

- prd.md
- requirements.json
- test_spec.json
- tasks.json

The specification is now ready for implementation. Add all new and updated files to `git` and commit them.

## Coding

Create an implementation plan:

```shell
af plan --spec 01_basic_svc
```

`agent-fox` analyses the spec and its dependencies and creates a task graph for the orchestrator:

```shell
Execution Plan
========================================
Specs:         01_basic_svc
Total tasks:   11
Review nodes:  3
Dependencies:  13
Fast mode:     off
Execution order:
    1. 01_basic_svc:0:reviewer:drift-review — Reviewer (drift-review)                                                                                              
    2. 01_basic_svc:0:reviewer:pre-review — Reviewer (pre-review)
    3. 01_basic_svc:1 — Write Integration and Property Tests
    4. 01_basic_svc:2 — Initialise Go Module and Project Layout                                                                                                    
    5. 01_basic_svc:3 — Implement Configuration Loading
    6. 01_basic_svc:4 — Implement Structured JSON Logging                                                                                                          
    7. 01_basic_svc:5 — Implement Database Initialisation and Schema                                                                                               
    8. 01_basic_svc:6 — Implement Bearer Token Authentication Middleware
    9. 01_basic_svc:7 — Implement POST /v1/events Handler                                                                                                          
    10. 01_basic_svc:8 — Implement Health Check Endpoints
    11. 01_basic_svc:9 — Implement Graceful Shutdown                                                                                                               
    12. 01_basic_svc:10 — Integration Test Suite Pass Checkpoint
    13. 01_basic_svc:11 — Wiring Verification and End-to-End Smoke Tests                                                                                           
    14. 01_basic_svc:0:verifier — Verifier Check
  ```

Make sure everything is committed by now, and merged into branch `develop`. 

Now start coding:

```shell
af code
```

`agent-fox` will implement the specification, one task at a time:

```shell
  /\_/\   _
  / o.o \/\ \
 ( > ^ < ) ) )
  \_^/\_/--'
agent-fox v4.0.0-rc1 (2b0a618).  model: claude-opus-4-6
/Users/candlekeep/devel/workspace/example-service
✔ 01_basic_svc:0:reviewer:drift-review [reviewer] done (1m 19s)
✔ 01_basic_svc:0:reviewer:pre-review [reviewer] done (2m 52s)
✔ 01_basic_svc:1 [coder] done (12m 13s)
✔ 01_basic_svc:2 [coder] done (2m 56s)
✔ 01_basic_svc:4 [coder] done (5m 5s)
✔ 01_basic_svc:5 [coder] done (5m 21s)                                                                                                                         
✔ 01_basic_svc:6 [coder] done (6m 0s)
✔ 01_basic_svc:7 [coder] done (3m 48s)
✔ 01_basic_svc:8 [coder] done (3m 14s)
✔ 01_basic_svc:9 [coder] done (7m 4s)
✔ 01_basic_svc:10 [coder] done (10m 48s)
✔ 01_basic_svc:11 [coder] done (7m 6s)
✔ 01_basic_svc:0:verifier [verifier] done (4m 3s)
Tasks:  14/14 done
Tokens: 35.5k in / 193.7k out
Cost:   $27.74
Status: completed

```
