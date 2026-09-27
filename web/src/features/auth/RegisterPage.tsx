import { UIWarning, UINotice } from "../../components/ui/FormControls";
import { Button, Card, Input, Label, TextField } from "@heroui/react";
import { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router";
import { useMutation } from "@tanstack/react-query";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { register } from "./api";
import { safeReturnTo, useAuth } from "./AuthProvider";
import { errorKey } from "./LoginPage";

export function RegisterPage() {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const navigate = useNavigate();
  const location = useLocation() as { state?: { from?: string } };
  const { onAuthenticated } = useAuth();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmValue, setConfirmValue] = useState("");
  const [reveal, setReveal] = useState(false);
  const [localError, setLocalError] = useState<Key | null>(null);

  const mutation = useMutation({
    mutationFn: () => register(name.trim(), email.trim(), password),
    onSuccess: async (user) => {
      await onAuthenticated(user);
      navigate(safeReturnTo(location.state?.from), { replace: true });
    },
  });

  useEffect(() => {
    document.title = "Judex · " + t("registerTitle");
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
        <Card.Title>{t("registerTitle")}</Card.Title>
        <Card.Description>{t("registerHint")}</Card.Description>
      </Card.Header>
      <Card.Content>
        <form className="judex-auth-form" onSubmit={(event) => {
          event.preventDefault();
          submit();
        }}>
          <TextField isRequired name="displayName" value={name} onChange={setName}>
            <Label>{t("fieldDisplayName")}</Label>
            <Input autoComplete="name" />
          </TextField>
          <TextField isRequired type="email" name="email" value={email} onChange={setEmail}>
            <Label>{t("fieldEmail")}</Label>
            <Input autoComplete="email" />
          </TextField>
          <TextField
            isRequired
            name="password"
            type={reveal ? "text" : "password"}
            value={password}
            onChange={setPassword}
          >
            <Label>{t("fieldPassword")}</Label>
            <Input autoComplete="new-password" />
          </TextField>
          <TextField
            isRequired
            name="passwordConfirm"
            type={reveal ? "text" : "password"}
            value={confirmValue}
            onChange={setConfirmValue}
          >
            <Label>{t("fieldPasswordConfirm")}</Label>
            <Input autoComplete="new-password" />
          </TextField>
          <label className="judex-auth-reveal">
            <input
              type="checkbox"
              checked={reveal}
              onChange={(event) => setReveal(event.target.checked)}
            />
            {reveal ? t("hidePassword") : t("showPassword")}
          </label>
          <small className="judex-auth-hint" aria-live="polite">{t("passwordRule")}</small>
          <Button type="submit" isPending={mutation.isPending}>
            {mutation.isPending ? t("submitting") : t("submitRegister")}
          </Button>
          {localError && <UIWarning role="alert">{t(localError)}</UIWarning>}
          {mutation.isError && (
            <UIWarning role="alert" aria-live="polite">
              {t(errorKey(mutation.error) ?? "errNetwork")}
            </UIWarning>
          )}
          {mutation.isSuccess && <UINotice>{t("registerDone")}</UINotice>}
        </form>
      </Card.Content>
      <Card.Footer>
        <Button variant="ghost" onClick={() => navigate("/login")}>
          {t("toLogin")}
        </Button>
      </Card.Footer>
    </Card>
  );
}
