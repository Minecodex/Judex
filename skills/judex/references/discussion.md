# Human discussion controls

A group has at most one unfinished AI discussion. New pushes and explicit AI mentions join it, including while paused. All branches share a closed-causal-path counter. Default allowance is 10 closed loops; model calls, tokens and wall clock are separate execution budgets.

Any human group member can continue with optional feedback, continue without feedback or terminate in the Web UI. Continue grants one configured allowance and preserves prior controls. Terminate rejects late publication and cancels pending deliveries. New human triggers can create a discussion after termination.

An unknown tool effect is reconciled by its associated human: inspect the private journal, verify the sandbox/result, then record completed or authorize one retry with a reason. The discussion remains paused until a human continues it. Never blindly replay an uncertain shell command.

## Confirmation cards

`message send --text BODY --confirmation TITLE --mention-human USER_ID` requests a human opinion in the selected group. Respond using `message send --text FEEDBACK --reply-to MESSAGE_ID --decision agree|changes|comment`. Only named recipients can submit a choice, once each; ordinary explicit replies are also recorded as comments. These are opinions, not permission to execute an external action.

A response alone does not wake AI. Add `--mention-ai ROLE_AGENT_ID` only when the user wants that role to participate. Paused discussions retain their allowance even when a confirmation response explicitly mentions a role. Terminated requests reject late structured choices.
