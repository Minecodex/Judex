# Backend boundaries

The Go/Gin application provides configuration, HTTP lifecycle, probes, request IDs, error responses, frontend hosting and real business services. `transport/http` adapts requests to application and domain services; it does not own authoritative business state. The Agent harness runs in server workers and dispatches tool commands to OpenSandbox containers.

Identity, project, work, decision, handoff, discussion, material, workflow, job and audit have implemented services, authorization, persistence and tests. Contracts remain in [the V1 plan](../docs/plans/v1/README.md); verified scope and remaining acceptance gates are tracked in [engineering status](../docs/详细设计/13-工程实现与验收状态.md).

Gin contexts must not enter domain services or background jobs. Authoritative state—including Agent sessions, runs and checkpoints—belongs in PostgreSQL only (no SQLite anywhere); the model harness runs in server worker processes, tool commands are dispatched to per-run OpenSandbox containers, and project shared files are mounted read-only into sandboxes.
