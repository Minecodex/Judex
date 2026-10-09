import { UIWarning } from "../../components/ui/FormControls";
import { Card } from "@heroui/react";
import { Button } from "../../components/ui/Button";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { request } from "../../lib/api/client";

type SessionSummary = {
  id: string;
  createdAt: string;
  expiresAt: string;
  lastSeenAt: string;
  current: boolean;
};

function formatTime(locale: string, iso: string): string {
  try {
    return new Intl.DateTimeFormat(locale === "en" ? "en" : "zh-CN", {
      dateStyle: "medium",
      timeStyle: "short",
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}

// 账户安全（docs/plans/v1/08 §4、P1-07）：会话列表与撤销；登出其他设备；
// 不出现任何假邮件按钮。设备授权列表在 P4-01 接入。
export function AccountSecurity() {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const client = useQueryClient();

  const sessions = useQuery({
    queryKey: ["me", "sessions"],
    queryFn: () => request<{ items: SessionSummary[] }>("/me/sessions?limit=50"),
  });

  const revoke = useMutation({
    mutationFn: (id: string) =>
      request(`/me/sessions/${id}`, { method: "DELETE" }),
    onSuccess: () => client.invalidateQueries({ queryKey: ["me", "sessions"] }),
  });

  useEffect(() => {
    document.title = "Judex · " + t("accountSecurityTitle");
  }, [locale]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <Card className="judex-account-card">
      <Card.Header>
        <Card.Title>{t("accountSecurityTitle")}</Card.Title>
        <Card.Description>{t("accountSecurityHint")}</Card.Description>
      </Card.Header>
      <Card.Content>
        {sessions.isPending ? (
          <p className="judex-workspace-status">{t("shellLoading")}</p>
        ) : sessions.isError ? (
          <UIWarning role="alert">{t("errServer")}</UIWarning>
        ) : (
          <ul className="judex-session-list">
            {(sessions.data?.items ?? []).map((session) => (
              <li key={session.id} className="judex-session-item">
                <div>
                  <strong>
                    {session.current ? t("sessionCurrent") : t("sessionOther")}
                  </strong>
                  <small>
                    {t("sessionLastSeen")}: {formatTime(locale, session.lastSeenAt)}
                    {" · "}
                    {t("sessionExpires")}: {formatTime(locale, session.expiresAt)}
                  </small>
                </div>
                {!session.current && (
                  <Button
                    variant="secondary"
                    isPending={revoke.isPending && revoke.variables === session.id}
                    onClick={() => revoke.mutate(session.id)}
                  >
                    {t("sessionRevoke")}
                  </Button>
                )}
              </li>
            ))}
            {(sessions.data?.items ?? []).length === 0 && (
              <li className="judex-session-item judex-workspace-status">
                {t("sessionNone")}
              </li>
            )}
          </ul>
        )}
      </Card.Content>
    </Card>
  );
}
