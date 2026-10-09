import {PreferencesPage as RolePreferencesPage} from "./PreferencesPage";
import {
  UICard,
  UIInput,
  UICheckbox,
  UITextArea,
  UIOption,
  UISelect,
} from "../../components/ui/FormControls";
import { isMySeat, myPreference } from "./preferences";
export { PreferencesPage } from "./PreferencesPage";
import { Button } from "../../components/ui/Button";
import {PositionAssignmentDialog, memberKey} from './PositionAssignmentDialog';
import {PositionCard} from './PositionCard';
import {CardActions} from '../../components/ui/ActionGroup';
import { useState } from "react";
import {
  ArrowUpRight,
  Bot,
  Check,
  Plus,
  UserPlus,
  Layers,
} from "lucide-react";
import { useWork } from "./store";
import { Btn, Dialog, EmptyState, Field, Heading, Person } from "./ui";
import { dataMode } from "../../lib/api/client";
import type { Position, Seat } from "./types";
import { PositionPresetsDialog } from "./PositionPresetsDialog";
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
            <UICheckbox appearance="card"
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
    flows = state.flows.filter((f) => f.projectId === project.id && f.status !== "draft");
  const [value, setValue] = useState({
    id: position?.id,
    name: position ? text(position.name) : "",
    prompt: position ? text(position.prompt) : "",
    publicSummary: position?.publicSummary ? text(position.publicSummary) : "",
    flowId: position?.bindings[0]?.flowId ?? (position ? "" : flows[0]?.id ?? ""),
    nodeId: position?.bindings[0]?.nodeId ?? (position ? "" : flows[0]?.nodes[0]?.id ?? ""),
  });
  const flow = flows.find((f) => f.id === value.flowId);
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
      <Field label={t("positionPresetSummary")}>
        <UIInput className="judex-input" data-testid="position-summary" value={value.publicSummary}
          onChange={e => setValue({...value, publicSummary: e.target.value})}/>
      </Field>
      {!flows.length && <p className="judex-muted">{t("positionPresetsNoFlow")}</p>}
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
                flowId: f?.id ?? "",
                nodeId: f?.nodes[0]?.id ?? "",
              });
            }}
          >
            <UIOption value="">{t("positionPresetsNoBinding")}</UIOption>
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
            disabled={!flow}
            onChange={(e) =>
              setValue({
                ...value,
                nodeId: e.target.value,
              })
            }
          >
            <UIOption value="">{t("positionPresetsNoBinding")}</UIOption>
            {flow?.nodes.map((n) => (
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
  const target = project.members.find(member => memberKey(member) === person);
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
            .filter((m) => seat.userId && m.userId ? m.userId !== seat.userId : m.name !== seat.person)
            .map((m) => (
              <UIOption key={memberKey(m)} value={memberKey(m)}>{m.name}{m.email ? ' · ' + m.email : ''}</UIOption>
            ))}
        </UISelect>
      </Field>
      <div className="judex-modal-actions">
        <Btn secondary onClick={onClose}>
          {t("cancel")}
        </Btn>
        <Btn
          disabled={!target}
          testId="confirm-seat-replace"
          onClick={async () => {
            if (
              (
                await act("replaceSeat", {
                  seatId: seat.id,
                  from: seat.person,
                  to: target!.name,
                  userId: target!.userId,
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
  const { state, project, route,t, text, management, go } = useWork(),
    [inviting, setInviting] = useState(false),
    [presetAdding, setPresetAdding] = useState(false),
    [assigning, setAssigning] = useState<Position | null>(null),
    [editing, setEditing] = useState<Position | "new" | null>(null),
    [replacing, setReplacing] = useState<Seat | null>(null),
    [viewing, setViewing] = useState<Seat | null>(null),[prefPosition,setPrefPosition]=useState<string>();
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
            <Btn secondary testId="open-position-presets" onClick={() => setPresetAdding(true)}>
              <Layers/>{t("positionPresetsOpen")}
            </Btn>
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
      <section className="judex-work-section">
        <div className="judex-work-section-title">
          <h2>{t("workPositions")}</h2>
          <small>{t("workPositionHint")}</small>
        </div>
        <div className="judex-position-grid">
          {positions.map(position => <PositionCard key={position.id} position={position}
            onEdit={() => setEditing(position)} onAssign={() => setAssigning(position)} onPreferences={()=>go({settingsItem:"preference:"+position.id})}/>)}
        </div>
      </section>
      {!!seats.length && <section className="judex-work-section"><div className="judex-work-section-title"><h2>{t('teamAssignedIdentities')}</h2></div>
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
              <CardActions className="judex-member-work-actions">
                <Button onClick={() => setViewing(seat)}>
                  {t("promptPreview")}
                  <ArrowUpRight />
                </Button>
                {management && (
                  <Button onClick={() => setReplacing(seat)}>
                    {t("workReplace")}
                  </Button>
                )}
              </CardActions>
            </UICard>
          );
        })}
      </div>
      </section>}
      {(route.settingsItem?.startsWith("preference:")||prefPosition)&&<Dialog wide title={t("coopPrivateRolePreferences")} onClose={()=>{setPrefPosition(undefined);go({settingsItem:undefined});}}><RolePreferencesPage initialPositionId={route.settingsItem?.startsWith("preference:")?route.settingsItem.slice(11):prefPosition} embedded/></Dialog>}
      {assigning && <PositionAssignmentDialog position={assigning} onClose={() => setAssigning(null)}/>}
      {inviting && <InviteDialog onClose={() => setInviting(false)} />}
      {presetAdding && <PositionPresetsDialog onClose={() => setPresetAdding(false)}/>}
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
              {isMySeat(state, viewing)
                ? myPreference(state, project.id, viewing.positionId)?.prompt || t("workNoPreference")
                : t("workPromptPrivate")}
            </p>
          </UICard>
          <p className="judex-muted">{t("workSimulated")}</p>
        </Dialog>
      )}
    </>
  );
}
