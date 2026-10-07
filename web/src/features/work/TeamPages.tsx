import {
  UICard,
  UIInput,
  UICheckbox,
  UITextArea,
  UIOption,
  UISelect,
} from "../../components/ui/FormControls";
import { useWorkspaceDraft } from "../chat/useWorkspaceDraft";
import { Button } from "../../components/ui/Button";
import { ExistingAssignments } from "./ProjectPages";
import { useState } from "react";
import {
  ArrowUpRight,
  Bot,
  Check,
  Plus,
  UserPlus,
  LockKeyhole,
  SlidersHorizontal,
} from "lucide-react";
import { useWork } from "./store";
import { Btn, Dialog, EmptyState, Field, Heading, Person } from "./ui";
import { dataMode } from "../../lib/api/client";
import type { Position, Seat } from "./types";
export function InvitePage() {
  const { state, project, t, text, act, go } = useWork();
  const invites = state.invites.filter(
    (i) =>
      i.projectId === project.id &&
      i.person === state.currentUser &&
      i.status === "pending",
  );
  return (
    <div className="judex-next-welcome">
      <span className="judex-welcome-flower">✳</span>
      <Heading
        title={t(invites.length ? "workWelcome" : "workNoAccess")}
        description={t(invites.length ? "workWelcomeSub" : "workNoAccessSub")}
      />
      {invites.map((invite) => (
        <UICard key={invite.id} className="judex-work-panel">
          <p>
            {invite.sender} → {state.currentUser}
          </p>
          {invite.positionIds.map((id) => {
            const p = state.positions.find((p) => p.id === id)!;
            return (
              <div className="judex-invited-position" key={id}>
                <Bot />
                <div>
                  <h3>{text(p.name)}</h3>
                  <p>{text(p.prompt)}</p>
                </div>
                <Check />
              </div>
            );
          })}
          <Btn
            testId="next-accept-invite"
            onClick={async () => {
              if ((await act("acceptInvite", { invitationId: invite.id })).ok)
                go({
                  view: "settings",
                });
            }}
          >
            {t("workAcceptInvite")}
          </Btn>
        </UICard>
      ))}
    </div>
  );
}
export function InviteDialog({ onClose }: { onClose: () => void }) {
  const { state, project, t, text, act } = useWork(),
    [name, setName] = useState(""),
    [ids, setIds] = useState<string[]>([]),
    [link, setLink] = useState("");
  return (
    <Dialog title={t("workInviteTitle")} onClose={onClose} wide>
      <p className="judex-modal-description">{t("workInviteHint")}</p>
      <Field label={t(dataMode === "api" ? "workInviteEmail" : "workInviteName")}>
        <UIInput
          className="judex-input"
          data-testid="next-invite-name"
          type={dataMode === "api" ? "email" : undefined}
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </Field>
      <h3>{t("workChoosePositions")}</h3>
      <div className="judex-position-options">
        {state.positions
          .filter((p) => p.projectId === project.id)
          .map((p) => (
            <UICheckbox
              key={p.id}
              data-testid={"invite-position-" + p.id}
              checked={ids.includes(p.id)}
              onChange={(e) =>
                setIds(
                  e.target.checked
                    ? [...ids, p.id]
                    : ids.filter((id) => id !== p.id),
                )
              }
            >
              <span>
                <strong>{text(p.name)}</strong>
                <small>
                  {state.seats
                    .filter((s) => s.positionId === p.id)
                    .map((s) => s.person)
                    .join(" · ")}
                </small>
              </span>
            </UICheckbox>
          ))}
      </div>
      {dataMode === "demo" && (
        <p className="judex-muted">{t("workInviteLocal")}</p>
      )}
      {link && (
        <Field label={t("workInviteLink")}>
          <UIInput className="judex-input" readOnly value={link} />
        </Field>
      )}
      <div className="judex-modal-actions">
        <Btn secondary onClick={onClose}>
          {t("cancel")}
        </Btn>
        <Btn
          testId="next-create-invite"
          disabled={dataMode === "api" && !name.includes("@")}
          onClick={async () => {
            const r = await act("invitePerson", {
              projectId: project.id,
              name,
              positionIds: ids,
            });
            if (r.ok) {
              if (r.inviteUrl) setLink(r.inviteUrl);
              else onClose();
            }
          }}
        >
          {t(dataMode === "api" ? "workSendInvite" : "workCreateInvite")}
        </Btn>
      </div>
    </Dialog>
  );
}
function PositionDialog({
  position,
  onClose,
}: {
  position?: Position;
  onClose: () => void;
}) {
  const { state, project, t, text, act } = useWork(),
    flows = state.flows.filter((f) => f.projectId === project.id);
  const [value, setValue] = useState({
    id: position?.id,
    name: position ? text(position.name) : "",
    prompt: position ? text(position.prompt) : "",
    flowId: position?.bindings[0]?.flowId ?? flows[0].id,
    nodeId: position?.bindings[0]?.nodeId ?? flows[0].nodes[0].id,
  });
  const flow = flows.find((f) => f.id === value.flowId)!;
  return (
    <Dialog
      title={t(position ? "workEditPosition" : "workCreatePosition")}
      onClose={onClose}
      wide
    >
      <p className="judex-modal-description">{t("workPositionHint")}</p>
      <Field label={t("workPositionName")}>
        <UIInput
          className="judex-input"
          data-testid="position-name"
          value={value.name}
          onChange={(e) =>
            setValue({
              ...value,
              name: e.target.value,
            })
          }
        />
      </Field>
      <Field label={t("workPositionPrompt")}>
        <UITextArea
          className="judex-textarea"
          data-testid="position-prompt"
          value={value.prompt}
          onChange={(e) =>
            setValue({
              ...value,
              prompt: e.target.value,
            })
          }
        />
      </Field>
      <div className="judex-work-form-grid">
        <Field label={t("workChooseFlow")}>
          <UISelect
            className="judex-input"
            data-testid="position-flow"
            value={value.flowId}
            onChange={(e) => {
              const f = flows.find((f) => f.id === e.target.value)!;
              setValue({
                ...value,
                flowId: f.id,
                nodeId: f.nodes[0].id,
              });
            }}
          >
            {flows.map((f) => (
              <UIOption key={f.id} value={f.id}>
                {text(f.name)}
              </UIOption>
            ))}
          </UISelect>
        </Field>
        <Field label={t("workChooseNode")}>
          <UISelect
            className="judex-input"
            data-testid="position-node"
            value={value.nodeId}
            onChange={(e) =>
              setValue({
                ...value,
                nodeId: e.target.value,
              })
            }
          >
            {flow.nodes.map((n) => (
              <UIOption value={n.id} key={n.id}>
                {text(n.label)}
              </UIOption>
            ))}
          </UISelect>
        </Field>
      </div>
      <div className="judex-modal-actions">
        <Btn secondary onClick={onClose}>
          {t("cancel")}
        </Btn>
        <Btn
          testId="save-position"
          onClick={async () => {
            if ((await act("savePosition", { projectId: project.id, value })).ok)
              onClose();
          }}
        >
          {t("workSavePosition")}
        </Btn>
      </div>
    </Dialog>
  );
}
function ReplaceDialog({ seat, onClose }: { seat: Seat; onClose: () => void }) {
  const { project, t, act } = useWork(),
    [person, setPerson] = useState("");
  return (
    <Dialog title={t("workReplace")} onClose={onClose}>
      <p className="judex-modal-description">{t("workReplaceHint")}</p>
      <Field label={t("workChooseSeat")}>
        <UISelect
          className="judex-input"
          data-testid="replace-seat-person"
          value={person}
          onChange={(e) => setPerson(e.target.value)}
        >
          <UIOption value="">{t("assignmentTarget")}</UIOption>
          {project.members
            .filter((m) => m.name !== seat.person)
            .map((m) => (
              <UIOption key={m.name}>{m.name}</UIOption>
            ))}
        </UISelect>
      </Field>
      <div className="judex-modal-actions">
        <Btn secondary onClick={onClose}>
          {t("cancel")}
        </Btn>
        <Btn
          testId="confirm-seat-replace"
          onClick={async () => {
            if (
              (
                await act("replaceSeat", {
                  seatId: seat.id,
                  from: seat.person,
                  to: person,
                })
              ).ok
            )
              onClose();
          }}
        >
          {t("confirm")}
        </Btn>
      </div>
    </Dialog>
  );
}
export function TeamPage() {
  const { state, project, t, text, management, go } = useWork(),
    [inviting, setInviting] = useState(false),
    [editing, setEditing] = useState<Position | "new" | null>(null),
    [replacing, setReplacing] = useState<Seat | null>(null),
    [viewing, setViewing] = useState<Seat | null>(null);
  const positions = state.positions.filter((p) => p.projectId === project.id),
    seats = state.seats.filter((s) =>
      positions.some((p) => p.id === s.positionId),
    );
  return (
    <>
      <Heading
        eyebrow="PEOPLE, WITH A PLACE"
        title={t("workTeam")}
        description={t("workMembersFirst")}
      >
        {management && (
          <>
            <Btn
              secondary
              onClick={() =>
                state.flows.some((f) => f.projectId === project.id)
                  ? setEditing("new")
                  : go({
                      view: "flows",
                    })
              }
              testId="new-position"
            >
              <Plus />
              {t("workCreatePosition")}
            </Btn>
            <Btn onClick={() => setInviting(true)} testId="next-invite">
              <UserPlus />
              {t("workInvite")}
            </Btn>
          </>
        )}
      </Heading>
      <ExistingAssignments />
      <div className="judex-work-team-grid">
        {seats.map((seat) => {
          const position = positions.find((p) => p.id === seat.positionId)!;
          const tasks = state.tasks.filter(
            (t) =>
              t.projectId === project.id &&
              (t.seatIds.includes(seat.id) || t.reviewerSeatId === seat.id),
          );
          return (
            <UICard
              className="judex-member-work-card"
              key={seat.id}
              data-testid={"seat-" + seat.id}
            >
              <Person seatId={seat.id} />
              <p>{text(seat.notes)}</p>
              <div className="judex-member-work-count">
                <span>{tasks.length}</span>
                {t("workTasks")}
                <small>{text(position.name)}</small>
              </div>
              <div className="judex-member-work-actions">
                <Button onClick={() => setViewing(seat)}>
                  {t("promptPreview")}
                  <ArrowUpRight />
                </Button>
                {management && (
                  <Button onClick={() => setReplacing(seat)}>
                    {t("workReplace")}
                  </Button>
                )}
              </div>
            </UICard>
          );
        })}
      </div>
      <section className="judex-work-section">
        <div className="judex-work-section-title">
          <h2>{t("workPositions")}</h2>
          <small>{t("workPositionHint")}</small>
        </div>
        <div className="judex-position-grid">
          {positions.map((p) => (
            <article key={p.id}>
              <span className={"judex-position-symbol judex-tone-" + p.tone}>
                <Bot />
              </span>
              <h3>{text(p.name)}</h3>
              <p>{text(p.prompt)}</p>
              <div className="judex-position-nodes">
                {p.bindings.map((b) => {
                  const f = state.flows.find((f) => f.id === b.flowId)!;
                  return (
                    <Button
                      key={b.flowId + b.nodeId}
                      onClick={() =>
                        go({
                          view: "flows",
                          id: f.id,
                        })
                      }
                    >
                      {text(f.name)} /{" "}
                      {text(f.nodes.find((n) => n.id === b.nodeId)!.label)}
                    </Button>
                  );
                })}
              </div>
              {management && (
                <Btn
                  secondary
                  onClick={() => setEditing(p)}
                  testId={"edit-position-" + p.id}
                >
                  <SlidersHorizontal />
                  {t("workEditPosition")}
                </Btn>
              )}
            </article>
          ))}
        </div>
      </section>
      {inviting && <InviteDialog onClose={() => setInviting(false)} />}
      {editing && (
        <PositionDialog
          position={editing === "new" ? undefined : editing}
          onClose={() => setEditing(null)}
        />
      )}
      {replacing && (
        <ReplaceDialog seat={replacing} onClose={() => setReplacing(null)} />
      )}
      {viewing && (
        <Dialog
          title={t("promptPreviewTitle")}
          onClose={() => setViewing(null)}
          wide
        >
          <Person seatId={viewing.id} />
          <UICard className="judex-work-callout">
            <h3>{t("workPositionPrompt")}</h3>
            <p>
              {text(positions.find((p) => p.id === viewing.positionId)!.prompt)}
            </p>
          </UICard>
          <UICard className="judex-work-callout">
            <h3>{t("workMyPrompt")}</h3>
            <p>
              {viewing.person === state.currentUser
                ? state.preferences.find(
                    (p) =>
                      p.projectId === project.id &&
                      p.person === state.currentUser,
                  )?.prompt || t("workNoPreference")
                : t("workPromptPrivate")}
            </p>
          </UICard>
          <p className="judex-muted">{t("workSimulated")}</p>
        </Dialog>
      )}
    </>
  );
}
export function PreferencesPage() {
  const { state, project, t, act, management, reset } = useWork(),
    value =
      state.preferences.find(
        (p) => p.projectId === project.id && p.person === state.currentUser,
      )?.prompt ?? "";
  const [prompt, setPrompt] = useWorkspaceDraft("personal-prompt", value),
    [resetting, setResetting] = useState(false);
  return (
    <>
      <Heading
        eyebrow="YOUR WAY OF WORKING"
        title={t("workMyPrompt")}
        description={t("workMyPromptHint")}
      />
      <div className="judex-work-preferences">
        <UICard className="judex-work-panel">
          <Person name={state.currentUser} />
          <Field label={t("workMyPrompt")}>
            <UITextArea
              className="judex-textarea judex-prompt-editor"
              data-testid="next-personal-prompt"
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder={t("workPromptExample")}
            />
          </Field>
          <Btn
            testId="next-save-preference"
            onClick={() =>
              void act("personalPrompt", { projectId: project.id, prompt })
            }
          >
            {t("personalSave")}
          </Btn>
        </UICard>
        <aside>
          <LockKeyhole />
          <h3>{t("personalPrivate")}</h3>
          <p>{t("personalHint")}</p>
          <p>{t("promptBoundary")}</p>
        </aside>
      </div>
      {dataMode === "demo" && (
        <div className="judex-next-demo-settings">
          <p>{t("workSimulated")}</p>
          {management && (
            <Btn
              secondary
              onClick={() => void act("remindPending", { projectId: project.id })}
              testId="simulate-receipt-reminder"
            >
              {t("workReminderDemo")}
            </Btn>
          )}
          <p>{t("workResetHint")}</p>
          {resetting ? (
            <div>
              <Btn danger testId="reset-next" onClick={reset}>
                {t("confirm")}
              </Btn>
              <Btn secondary onClick={() => setResetting(false)}>
                {t("cancel")}
              </Btn>
            </div>
          ) : (
            <Btn secondary onClick={() => setResetting(true)}>
              {t("workReset")}
            </Btn>
          )}
        </div>
      )}
    </>
  );
}
