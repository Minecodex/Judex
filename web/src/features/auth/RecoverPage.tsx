import { UINotice, UIWarning } from "../../components/ui/FormControls";
import { Card, FieldError, Input, Label, TextField } from "@heroui/react";
import { Button } from "../../components/ui/Button";
import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { useMutation } from "@tanstack/react-query";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { APIError } from "../../lib/api/client";
import { recover } from "./api";
import { errorKey } from "./errors";
import { validatePassword } from "./validation";

type FieldErrors = { code?: Key | null; password?: Key | null; confirm?: Key | null };

// /recover（docs/plans/v1/08 §4）：输入运维一次性恢复码与新密码；无 SMTP，
// 页面如实展示获取方式，不虚构邮件发送。成功后不自动登录。
export function RecoverPage() {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const navigate = useNavigate();
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [confirmValue, setConfirmValue] = useState("");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});

  const mutation = useMutation({
    mutationFn: () => recover(code.trim(), password),
  });

  useEffect(() => {
    document.title = "Judex · " + t("recoverTitle");
  }, [locale]); // eslint-disable-line react-hooks/exhaustive-deps

  const clear = (field: keyof FieldErrors) =>
    setFieldErrors((prev) => ({ ...prev, [field]: null }));

  const submit = () => {
    const errors: FieldErrors = {
      code: code.trim() ? null : "errRecoveryCodeRequired",
      password: validatePassword(password),
      confirm: password !== confirmValue ? "errMismatch" : null,
    };
    if (errors.code || errors.password || errors.confirm) {
      setFieldErrors(errors);
      return;
    }
    setFieldErrors({});
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
          <TextField
            isRequired
            name="recoveryCode"
            value={code}
            onChange={(v) => { setCode(v); clear("code"); }}
            isInvalid={!!fieldErrors.code}
          >
            <Label>{t("fieldRecoveryCode")}</Label>
            <Input autoComplete="one-time-code" />
            <FieldError>{fieldErrors.code ? t(fieldErrors.code) : ""}</FieldError>
          </TextField>
          <TextField
            isRequired
            name="newPassword"
            type="password"
            value={password}
            onChange={(v) => { setPassword(v); clear("password"); }}
            isInvalid={!!fieldErrors.password}
          >
            <Label>{t("fieldNewPassword")}</Label>
            <Input autoComplete="new-password" />
            <FieldError>{fieldErrors.password ? t(fieldErrors.password) : ""}</FieldError>
          </TextField>
          <TextField
            isRequired
            name="confirmPassword"
            type="password"
            value={confirmValue}
            onChange={(v) => { setConfirmValue(v); clear("confirm"); }}
            isInvalid={!!fieldErrors.confirm}
          >
            <Label>{t("fieldPasswordConfirm")}</Label>
            <Input autoComplete="new-password" />
            <FieldError>{fieldErrors.confirm ? t(fieldErrors.confirm) : ""}</FieldError>
          </TextField>
          <Button variant="primary" fullWidth size="lg" type="submit" isPending={mutation.isPending}>
            {mutation.isPending ? t("submitting") : t("submitRecover")}
          </Button>
          {mutation.isError && (
            <UIWarning role="alert" aria-live="polite">
              {mutation.error instanceof APIError &&
              (mutation.error.code === "UNAUTHENTICATED" || mutation.error.status === 401)
                ? t("errRecoveryInvalid")
                : t(errorKey(mutation.error) ?? "errNetwork")}
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
