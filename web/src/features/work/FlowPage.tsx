import {
  UIInput,
  UITextArea,
  UIDisclosure,
  UICard,
  UICheckbox,
} from "../../components/ui/FormControls";
import { useWorkspaceDraft } from "../chat/useWorkspaceDraft";
import { Button } from "../../components/ui/Button";
import { useState } from "react";
import { Check, GitBranch, Plus, Sparkles } from "lucide-react";
import { useWork } from "./store";
import { Btn, Dialog, Field, Heading, EmptyState } from "./ui";
import type { Flow } from "./types";
import {WorkflowDiagram as FlowDiagram,workflowCode as flowCode} from "./WorkflowDiagram";
import {WorkflowPresetsDialog} from "./WorkflowPresetsDialog";
import {WorkflowDraftEditor} from "./WorkflowDraftEditor";
import {UIDialog} from "../../components/ui/Presentation";
import {UIStatus} from "../../components/ui/FormControls";
function NewFlowDialog({ onClose }: { onClose: () => void }) {
  const { t, project, act } = useWork(),
    [name, setName] = useState(""),
    [instructions, setInstructions] = useState(""),
    [steps, setSteps] = useState("");
  return (
    <Dialog title={t("workNewFlow")} onClose={onClose} wide>
      <p className="judex-modal-description">{t("workFlowSim")}</p>
      <Field label={t("workTitle")}>
        <UIInput
          className="judex-input"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </Field>
      <Field label={t("workFlowInput")}>
        <UITextArea
          className="judex-textarea"
          value={instructions}
          onChange={(e) => setInstructions(e.target.value)}
        />
      </Field>
      <Field label={t("workFlowSteps")}>
        <UITextArea
          className="judex-textarea judex-textarea-short"
          value={steps}
          onChange={(e) => setSteps(e.target.value)}
        />
      </Field>
      <div className="judex-modal-actions">
        <Btn secondary onClick={onClose}>
          {t("cancel")}
        </Btn>
        <Btn
          onClick={async () => {
            const r = await act("createFlow", {
              projectId: project.id,
              name,
              instructions,
              labels: steps
                .split("\n")
                .map((v) => v.trim())
                .filter(Boolean),
            });
            if (r.ok) onClose();
          }}
        >
          {t("workFlowPublish")}
        </Btn>
      </div>
    </Dialog>
  );
}
export function FlowPage() {
  const { state, project, t, text, locale, management, route, go, act } =
      useWork(),
    flows = state.flows.filter((f) => f.projectId === project.id),
    flow = flows.find((f) => f.id === route.id) ?? flows[0],
    [creating, setCreating] = useState(false),
    [templates,setTemplates] = useState(false), [editing,setEditing] = useState(false);
  return (
    <>
      <Heading
        eyebrow="A SHARED WAY FORWARD"
        title={t("workFlows")}
        description={t("workFlowHint")}
      >
        {management && (
          <><Button variant="primary" data-testid="open-workflow-presets" onPress={() => setTemplates(true)}><Plus/>{t("workflowPresetsOpen")}</Button><Btn secondary onClick={() => setCreating(true)}>
            <Plus />
            {t("workNewFlow")}
          </Btn></>
        )}
      </Heading>
      <div className="judex-work-segments judex-work-filter">
        {flows.map((f) => (
          <Button
            key={f.id}
            aria-pressed={f.id === flow.id}
            onClick={() =>
              go({
                view: "flows",
                id: f.id,
              })
            }
          >
            {text(f.name)}
          </Button>
        ))}
      </div>
      {flow ? (
        <div className={'judex-flow-layout'+(flow.status === 'draft'?' judex-flow-layout-draft':'')}>
          <section className="judex-flow-main">
            <div className="judex-flow-title">
              <GitBranch />
              <h2>{text(flow.name)}</h2>
              <span>v{flow.version}</span><UIStatus>{t(flow.status === "draft" ? "workflowDraft" : "workflowPublished")}</UIStatus>
              {management && flow.status !== "draft" && <Button size="sm" data-testid="edit-workflow" onPress={() => setEditing(true)}>{t("workflowEdit")}</Button>}
            </div>
            {flow.status === "draft" ? (flow.body ? <WorkflowDraftEditor key={flow.id} flow={flow}/> : <p role="status">{t('workflowDiagramLoading')}</p>) : <FlowDiagram flow={flow} /> }
            <p className="judex-work-small-note">{t("workFlowLabels")}</p>
            <div className="judex-flow-node-notes">
              {flow.nodes.map((node) => (
                <article key={node.id}>
                  <span>{flow.nodes.indexOf(node) + 1}</span>
                  <div>
                    <h3>{text(node.label)}</h3>
                    <p>
                      {state.positions
                        .filter((p) =>
                          p.bindings.some(
                            (b) => b.flowId === flow.id && b.nodeId === node.id,
                          ),
                        )
                        .map((p) => text(p.name))
                        .join(" · ") || t("workNoItems")}
                    </p>
                  </div>
                </article>
              ))}
            </div>
            <UIDisclosure
              className="judex-work-records"
              title={<>{t("workFlowMermaid")}</>}
            >
              <pre>{flowCode(flow, locale === "en")}</pre>
            </UIDisclosure>
            <UICard className="judex-work-callout">
              <h3>{t("workFlowCurrent")}</h3>
              <p>{text(flow.instructions)}</p>
            </UICard>
          </section>
          {flow.status !== "draft" && <FlowEditor
            key={flow.id + ":" + flow.version + ":" + state.currentUser}
            flow={flow}
          />}
        </div>
      ) : (
        <EmptyState text={t("workNoItems")} />
      )}
      {creating && <NewFlowDialog onClose={() => setCreating(false)} />}
      {templates && <WorkflowPresetsDialog onClose={() => setTemplates(false)}/>}
      {editing && flow && <UIDialog title={t("workflowEdit")} wide onClose={() => setEditing(false)}><WorkflowDraftEditor key={flow.id+":"+flow.status} flow={flow}/></UIDialog>}
    </>
  );
}
function FlowEditor({ flow }: { flow: Flow }) {
  const { t, text, project, management, act } = useWork(),
    [input, setInput] = useWorkspaceDraft(
      "flow-input:" + flow.id + ":" + flow.version,
      "",
    ),
    [draft, setDraft] = useWorkspaceDraft(
      "flow-draft:" + flow.id + ":" + flow.version,
      "",
    ),
    [impact, setImpact] = useWorkspaceDraft(
      "flow-impact:" + flow.id + ":" + flow.version,
      false,
    );
  return (
    <aside className="judex-flow-editor">
      <span className="judex-ai-mark">
        <Sparkles />
      </span>
      <h2>{t("workFlowChat")}</h2>
      <p>{t("workFlowSim")}</p>
      {management ? (
        <>
          <Field label={t("workFlowInput")}>
            <UITextArea
              className="judex-textarea"
              data-testid="workflow-input"
              value={input}
              onChange={(e) => setInput(e.target.value)}
            />
          </Field>
          <Btn
            secondary
            disabled={!input.trim()}
            testId="prepare-flow-draft"
            onClick={() =>
              setDraft(text(flow.instructions) + "\n\n" + input.trim())
            }
          >
            <Sparkles />
            {t("workFlowDraft")}
          </Btn>
          {draft && (
            <>
              <Field label={t("workFlowCurrent")}>
                <UITextArea
                  className="judex-textarea"
                  data-testid="workflow-draft"
                  value={draft}
                  onChange={(e) => setDraft(e.target.value)}
                />
              </Field>
              <UICheckbox
                className="judex-work-checkbox"
                data-testid="flow-material-change"
                checked={impact}
                onChange={(e) => setImpact(e.target.checked)}
              >
                <span>{t("workFlowImpact")}</span>
              </UICheckbox>
              <Btn
                testId="publish-flow"
                onClick={() =>
                  void act("publishFlow", {
                    projectId: project.id,
                    flowId: flow.id,
                    version: flow.version,
                    instructions: draft,
                    material: impact,
                  })
                }
              >
                <Check />
                {t("workFlowPublish")}
              </Btn>
            </>
          )}
        </>
      ) : (
        <UICard className="judex-work-callout">{t("workFlowReadOnly")}</UICard>
      )}
      <div className="judex-flow-boundary">
        <strong>{t("workHard")}</strong>
        <p>{t("workConditionHint")}</p>
      </div>
    </aside>
  );
}
