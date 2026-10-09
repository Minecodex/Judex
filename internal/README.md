# Backend boundaries

The runnable scaffold implements configuration, HTTP lifecycle, probes, request IDs, error responses and frontend hosting. `transport/http` contains no business state. `agent` defines the execution boundary only; it never executes commands on the API host.

Modules are reserved for identity, project, work, decision, handoff, discussion, material, workflow, job and audit. Their implementations, authorization, PostgreSQL repositories, migrations, S3 adapter and OpenSandbox client are being built per [docs/plans/v1](../docs/plans/v1/README.md); until a module lands, its placeholder APIs respond with HTTP 501, never simulated success.

Gin contexts must not enter domain services or background jobs. Authoritative state—including Agent sessions, runs and checkpoints—belongs in PostgreSQL only (no SQLite anywhere); the model harness runs in server worker processes, tool commands are dispatched to per-run OpenSandbox containers, and project shared files are mounted read-only into sandboxes.
