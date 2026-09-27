import { UIWarning, UINotice } from "../../components/ui/FormControls";
import { Button, Card, Input, Label, TextField } from "@heroui/react";
import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { useMutation } from "@tanstack/react-query";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { APIError } from "../../lib/api/client";
import { recover } from "./api";

// /recover（docs/plans/v1/08 §4）：输入运维一次性恢复码与新密码；无 SMTP，
// 页面如实展示获取方式，不虚构邮件发送。成功后不自动登录。
export function RecoverPage() {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const navigate = useNavigate();
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [confirmValue, setConfirmValue] = useState("");
  const [localError, setLocalError] = useState<Key | null>(null);

  const mutation = useMutation({
    mutationFn: () => recover(code.trim(), password),
  });

  useEffect(() => {
    document.title = "Judex · " + t("recoverTitle");
  }, [locale]); // eslint-disable-line react-hooks/exhaustive-deps

  const submit = () => {
    if (password !== confirmValue) {
      setLocalError("errMismatch");
      return;
    }
    setLocalError(null);
    mutation.mutate();
  };

  return (
    <Card className="judex-auth-card">
      <Card.Header>
        <Card.Title>{t("recoverTitle")}</Card.Title>
        <Card.Description>{t("recoverHint")}</Card.Description>
      </Card.Header>
      <Card.Content>
        <small className="judex-auth-hint">{t("recoverNote")}</small>
        <form className="judex-auth-form" onSubmit={(event) => {
          event.preventDefault();
          submit();
        }}>
          <TextField isRequired name="recoveryCode" value={code} onChange={setCode}>
            <Label>{t("fieldRecoveryCode")}</Label>
            <Input autoComplete="one-time-code" />
          </TextField>
          <TextField
            isRequired
            name="newPassword"
            type="password"
            value={password}
            onChange={setPassword}
          >
            <Label>{t("fieldNewPassword")}</Label>
            <Input autoComplete="new-password" />
          </TextField>
          <TextField
            isRequired
            name="confirmPassword"
            type="password"
            value={confirmValue}
            onChange={setConfirmValue}
          >
            <Label>{t("fieldPasswordConfirm")}</Label>
            <Input autoComplete="new-password" />
          </TextField>
          <Button type="submit" isPending={mutation.isPending}>
            {mutation.isPending ? t("submitting") : t("submitRecover")}
          </Button>
          {localError && <UIWarning role="alert">{t(localError)}</UIWarning>}
          {mutation.isError && (
            <UIWarning role="alert" aria-live="polite">
              {mutation.error instanceof APIError &&
              (mutation.error.code === "UNAUTHENTICATED" || mutation.error.status === 401)
                ? t("errRecoveryInvalid")
                : t("errNetwork")}
            </UIWarning>
          )}
          {mutation.isSuccess && (
            <UINotice>
              {t("recoverDone")}{" "}
              <Button variant="ghost" onClick={() => navigate("/login")}>
                {t("backToLogin")}
              </Button>
            </UINotice>
          )}
        </form>
      </Card.Content>
    </Card>
  );
}
