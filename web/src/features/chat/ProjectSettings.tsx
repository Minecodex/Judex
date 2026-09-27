import { UIWarning, UICard } from "../../components/ui/FormControls";
import { useWorkspaceDraft } from "./useWorkspaceDraft";
import { Input } from "@heroui/react";
import { Sparkles } from "lucide-react";
import { useWork } from "../work/store";
import { Btn, Field, Heading } from "../work/ui";
import {
  roundLimit,
  validRoundLimit,
  setDiscussionLimit,
  MAX_DISCUSSION_ROUNDS,
} from "./discussionPolicy";
export function ProjectSettings() {
  const { project, t, act, management } = useWork();
  const [value, setValue] = useWorkspaceDraft(
    "project-round-limit",
    String(roundLimit(project)),
  );
  const valid = value.trim() !== "" && validRoundLimit(Number(value));
  return (
    <section className="judex-project-ai-settings">
      <Heading title={t("chatSettings")} description={t("chatProjectAIHint")} />
      <UICard className="judex-work-panel">
        <Sparkles size={22} />
        <h2>{t("chatDiscussionPolicy")}</h2>
        <Field label={t("chatDiscussionLimit")}>
          <Input
            type="number"
            min={1}
            max={MAX_DISCUSSION_ROUNDS}
            step={1}
            aria-label={t("chatDiscussionLimit")}
            data-testid="project-discussion-limit"
            value={value}
            disabled={!management}
            aria-invalid={!valid}
            onChange={(e) => setValue(e.target.value)}
          />
        </Field>
        <p>{t("chatRoundMeaning")}</p>
        <p>{t("chatRoundSnapshot")}</p>
        {!valid && (
          <UIWarning role="alert">{t("chatRoundValidation")}</UIWarning>
        )}
        {management ? (
          <Btn
            disabled={!valid}
            testId="save-discussion-limit"
            onClick={() =>
              act((s) => setDiscussionLimit(s, project.id, Number(value)))
            }
          >
            {t("chatSaveSettings")}
          </Btn>
        ) : (
          <p>{t("chatSettingsReadOnly")}</p>
        )}
      </UICard>
      <UICard className="judex-work-callout">{t("chatRoundEarlyStop")}</UICard>
    </section>
  );
}
