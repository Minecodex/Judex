import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import { Card } from "@heroui/react";
import { Button } from "../../components/ui/Button";
import { FormField, UIInput, UICheckbox, UIWarning, UINotice } from "../../components/ui/FormControls";
import { request } from "../../lib/api/client";
import { listProjects } from "./api";
import { useAuth } from "./AuthProvider";
import { translate, type Key } from "../../i18n";
import { usePreferences } from "../../stores/preferences";

type DeviceReview = { deviceName: string; scopes: string[]; projectScope: string[]; state: string; expiresAt: string };
export function DevicePage() {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const { user } = useAuth();
  const [code, setCode] = useState(new URLSearchParams(location.search).get("userCode") ?? "");
  const [reviewCode, setReviewCode] = useState("");
  const [scopes, setScopes] = useState<string[]>([]);
  const [projectScope, setProjectScope] = useState<string[]>([]);
  const projects = useQuery({ queryKey: ["projects", user?.id], queryFn: listProjects });
  const review = useQuery({
    queryKey: ["deviceReview", user?.id, reviewCode], enabled: !!reviewCode, retry: false,
    queryFn: () => request<DeviceReview>(`/auth/device/review?userCode=${encodeURIComponent(reviewCode)}`),
  });
  const confirm = useMutation({
    mutationFn: (approved: boolean) => request("/auth/device/confirm", {
      method: "POST", body: JSON.stringify({ userCode: reviewCode, approved, scopes, projectScope }),
      idempotencyKey: crypto.randomUUID(),
    }),
  });
  const toggle = (values: string[], id: string) => values.includes(id) ? values.filter((v) => v !== id) : [...values, id];
  return <Card><Card.Content><div className="judex-panel-stack">
    <h2>{t("accessDevice")}</h2><p>{t("accessAccount")}: {user?.email}</p>
    <form className="judex-auth-form" onSubmit={(e) => { e.preventDefault(); confirm.reset(); setReviewCode(code.trim()); }}>
      <FormField label={t("accessCode")}><UIInput value={code} onChange={(e) => setCode(e.target.value)} /></FormField>
      <Button variant="primary" type="submit" disabled={!code.trim()}>{t("accessReview")}</Button>
    </form>
    {review.isError && <UIWarning>{t("accessUnavailable")}</UIWarning>}
    {review.data && <>
      <p>{t("accessDeviceName")}: {review.data.deviceName}</p>
      <p>{t("accessExpires")}: {new Date(review.data.expiresAt).toLocaleString()}</p>
      <p>{t("accessState")}: {review.data.state}</p>
      <strong>{t("accessScopes")}</strong>
      {review.data.scopes.map((scope) => <UICheckbox key={scope} checked={scopes.includes(scope)} onChange={() => setScopes(toggle(scopes, scope))}>{scope}</UICheckbox>)}
      <strong>{t("accessProjects")}</strong><p>{t("accessSelectProjects")}</p>
      {(projects.data ?? []).filter((p) => !review.data.projectScope?.length || review.data.projectScope.includes(p.id)).map((p) =>
        <UICheckbox key={p.id} checked={projectScope.includes(p.id)} onChange={() => setProjectScope(toggle(projectScope, p.id))}>{p.name}</UICheckbox>)}
      {confirm.isSuccess ? <UINotice>{t("accessDone")}</UINotice> : <div className="judex-inline-form">
        <Button variant="primary" isPending={confirm.isPending} disabled={review.data.state !== "pending" || !scopes.length} onPress={() => confirm.mutate(true)}>{t("accessApprove")}</Button>
        <Button variant="danger" disabled={review.data.state !== "pending"} onPress={() => confirm.mutate(false)}>{t("accessDeny")}</Button>
      </div>}
    </>}
    {confirm.isError && <UIWarning>{String(confirm.error.message)}</UIWarning>}
  </div></Card.Content></Card>;
}
