import { UIWarning, UINotice } from "../../components/ui/FormControls";
import { Button, Card } from "@heroui/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useParams } from "react-router";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { request } from "../../lib/api/client";
import { useAuth } from "./AuthProvider";
import { errorText } from "../projects/ProjectWorkspace";

type Intent = {
  id: string;
  state: string;
  operation: string;
  objectId: string;
  expiresAt: string;
  resultRef: string | null;
};

// ConfirmPage（docs/plans/v1/07 §5 / 08 §5）：CLI 请求的正式决定在此由本人
// 浏览器一次确认；页面只展示服务端事实，最终权限在后端。
export function ConfirmPage() {
  const { intentId } = useParams<{ intentId: string }>();
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const { user } = useAuth();
  const [projectId, setProjectId] = useState<string>(
    () => new URLSearchParams(location.search).get("project") ?? "",
  );

  const intent = useQuery({
    queryKey: ["intent", intentId],
    queryFn: () =>
      request<Intent>(`/projects/${projectId}/confirmation-intents/${intentId}`),
    enabled: !!projectId && !!intentId,
    refetchInterval: (query) =>
      query.state.data?.state === "pending" ? 3000 : false,
  });

  const decide = useMutation({
    mutationFn: (approve: boolean) =>
      request(`/projects/${projectId}/confirmation-intents/${intentId}/confirm`, {
        method: "POST",
        body: JSON.stringify({ decision: approve ? "approve" : "reject" }),
        idempotencyKey: crypto.randomUUID(),
      }),
    onSuccess: () => intent.refetch(),
  });

  useEffect(() => {
    document.title = "Judex · " + t("cfTitle");
  }, [locale]); // eslint-disable-line react-hooks/exhaustive-deps

  if (!projectId) {
    return (
      <EntryCard>
        <p className="judex-workspace-status">{t("cfNeedProject")}</p>
      </EntryCard>
    );
  }

  return (
    <EntryCard>
      <h2 className="judex-cf-title">{t("cfTitle")}</h2>
      {intent.isPending ? (
        <p className="judex-workspace-status">{t("shellLoading")}</p>
      ) : intent.isError ? (
        <UIWarning role="alert">{errorText(locale, intent.error)}</UIWarning>
      ) : (
        <div className="judex-panel-stack">
          <small>
            {t("cfAccount")}: {user?.displayName ?? "-"}
          </small>
          <small>
            {t("cfOperation")}: {intent.data?.operation}
          </small>
          <small>
            {t("cfObject")}: {intent.data?.objectId}
          </small>
          <small>
            {t("cfExpires")}:{" "}
            {intent.data?.expiresAt
              ? new Date(intent.data.expiresAt).toLocaleString()
              : "-"}
          </small>
          {intent.data?.state === "pending" ? (
            <div className="judex-inline-form">
              <Button
                variant="primary"
                data-testid="confirm-approve"
                isPending={decide.isPending}
                onClick={() => decide.mutate(true)}
              >
                {t("cfApprove")}
              </Button>
              <Button
                variant="danger"
                data-testid="confirm-reject"
                isPending={decide.isPending}
                onClick={() => decide.mutate(false)}
              >
                {t("cfReject")}
              </Button>
            </div>
          ) : (
            <UINotice>
              {t("cfState")}: {intent.data?.state}
              {intent.data?.resultRef ? ` · ${intent.data.resultRef}` : ""}
            </UINotice>
          )}
          {decide.isError && <UIWarning role="alert">{errorText(locale, decide.error)}</UIWarning>}
          <small className="judex-auth-hint">{t("cfHint")}</small>
        </div>
      )}
    </EntryCard>
  );
}

function EntryCard({ children }: { children: React.ReactNode }) {
  return (
    <main className="judex-entry min-h-screen flex items-center justify-center p-8">
      <Card className="judex-auth-card">
        <Card.Content>{children}</Card.Content>
      </Card>
    </main>
  );
}
