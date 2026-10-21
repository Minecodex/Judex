# V2 commands

All commands support `--server`, `--profile`, `--json`. Project/group context can be overridden with `--project`, `--group`. Machine output uses `{schemaVersion:"2",ok,data,error?}`.

```sh
judex auth login --server http://127.0.0.1:18097
judex project list
judex project use PROJECT_ID
judex group list
judex group use GROUP_ID
judex group show
judex message list --before SEQUENCE --search TEXT
judex message send --text TEXT
judex message send --text TEXT --mention-ai ROLE_AGENT_ID
judex file list
judex file upload ./notes.md
judex file download VERSION_ID ./notes.md
judex context read --after SEQUENCE
judex context read --kind message --ref MESSAGE_ID --offset 0 --limit 8000
judex push --text TEXT --file ./notes.md
judex push --record-only --text TEXT
judex run list
judex watch --after SEQUENCE
judex inbox
judex pending
```

No plan, task, handoff, delivery, approval or acceptance commands exist in V2. New and old clients must not be mixed.

A file-bearing `push` or `message send` saves one recovery operation, including confirmed attachment versions and the exact final message. After an uncertain failure, repeat the unchanged command: it resumes that operation across CLI processes. `pending` lists unfinished operations. Do not change files or flags while confirming a previous result. Once confirmed, another invocation without `--request-id` is a new upload/message. Supply one stable UUID with `--request-id` to replay the same explicit intent even after confirmation; use a different UUID for different content. Server authorization is checked again on replay.
