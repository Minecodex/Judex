import { Button } from "../../components/ui/Button";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { useBootstrap, useTopics, type ApiTopic } from "./api";

// 概览会话：项目首屏（bootstrap 概览 + 待办 + 最近议题）。
export function HomeConversation({
  projectId,
  onOpenTopic,
}: {
  projectId: string;
  onOpenTopic: (tab: { kind: "topic"; id: string }) => void;
}) {
  const { locale } = usePreferences();
  const t = (key: Key, values?: Record<string, string | number>) =>
    translate(locale, key, values);
  const bootstrap = useBootstrap(projectId);
  const topics = useTopics(projectId);

  if (bootstrap.isPending) {
    return <p className="judex-workspace-status">{t("wsLoading")}</p>;
  }
  if (bootstrap.isError) {
    return (
      <p className="judex-workspace-status" role="alert">
        {t("wsLoadFailed")}{" "}
        <Button variant="ghost" onClick={() => bootstrap.refetch()}>
          {t("wsRetry")}
        </Button>
      </p>
    );
  }
  const recent: ApiTopic[] = (topics.data?.items ?? []).slice(0, 6);
  const pending = bootstrap.data?.pendingActionsCount ?? 0;

  return (
    <div className="judex-home-conversation" data-testid="ws-home">
      <section className="judex-home-hero">
        <h2>{t("wsOverviewTitle")}</h2>
        <p>{bootstrap.data?.project.title}</p>
        <div className="judex-home-pending" data-testid="ws-pending">
          {pending > 0 ? t("wsPendingActions", { count: pending }) : t("wsNoPending")}
        </div>
      </section>
      <section className="judex-home-recent">
        <h3>{t("wsRecentTopics")}</h3>
        {recent.length ? (
          <ul>
            {recent.map((topic) => (
              <li key={topic.id}>
                <Button
                  variant="ghost"
                  onClick={() => onOpenTopic({ kind: "topic", id: topic.id })}
                  data-testid={"ws-recent-" + topic.id}
                >
                  {topic.title}
                </Button>
              </li>
            ))}
          </ul>
        ) : (
          <p className="judex-chat-empty-hint">{t("wsTopicsEmpty")}</p>
        )}
      </section>
    </div>
  );
}
