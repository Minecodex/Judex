---
name: judex
description: Read an authorized Judex V2 group and return explicitly selected local progress or files to that group using the native CLI.
---

# Judex V2 local collaboration

Use the installed `judex` CLI, protocol 2. Start with `judex version` and `judex status`. Use device authorization (`judex auth login --server URL`); browser consent selects the project scope. Never read credentials.json, print access/refresh tokens, or use another account.

Select the requested project and joined group with `project use ID` and `group use ID`. Read `group show`, `message list` and `context read` before acting. A group is a conversation with independent membership, not a plan/task/approval state machine. Hidden parent groups must not be inferred or queried.

Only push the local text/files the user authorized to the selected group. `judex push --text TEXT --file PATH` saves progress and lets the server wake all of the caller's roles in that group. Do not select a single identity or invent an author/source. Use `--record-only` when asked to record without waking AI. If no role is assigned, content remains saved and the server returns a clear warning.

Ordinary `message send` does not wake AI. `--mention-ai ROLE_AGENT_ID` is an explicit structured mention; `--mention-human USER_ID` only notifies the person. Discover exact IDs from `group show`. Do not scan plain @ text or guess role IDs. Personal chat has no AI.

Files are immutable versions. Normal same-name uploads create independent files. An explicit `file upload PATH --update FILE_ID --expected-version VERSION` creates a new version of an owned file. Downloads take a fixed version ID. Never expose unselected local files or broaden sharing.

Read references in pages using `context read --kind KIND --ref REF_ID --offset N --limit N`. Follow conversation events with `watch --after SEQUENCE`; reconnect using the returned cursor. Preserve network retry receipts and idempotency keys. Permissions are rechecked on every call; after removal stop reading/pushing rather than looking for a bypass.

Humans control paused discussions in the Web UI. Do not claim work is approved, accepted or completed by a platform workflow. Unknown shell effects require human verification before retry; a continue action does not prove a previous tool had no effect.

References: [commands](references/commands.md), [local reporting](references/reporting.md), [discussion controls](references/discussion.md).
