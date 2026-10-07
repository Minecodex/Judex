import { UIWarning } from "../../components/ui/FormControls";
import { Card, Input, Label, TextField } from "@heroui/react";
import { Button } from "../../components/ui/Button";
import { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router";
import { useMutation } from "@tanstack/react-query";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { login } from "./api";
import { errorKey } from "./errors";
import { safeReturnTo, useAuth } from "./AuthProvider";

export function LoginPage() {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const navigate = useNavigate();
  const location = useLocation() as { state?: { from?: string; expired?: boolean } };
  const { onAuthenticated } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [reveal, setReveal] = useState(false);

  const mutation = useMutation({
    mutationFn: () => login(email.trim(), password),
    onSuccess: async (user) => {
      await onAuthenticated(user);
      navigate(safeReturnTo(location.state?.from), { replace: true });
    },
  });

  useEffect(() => {
    document.title = "Judex · " + t("loginTitle");
  }, [locale]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <Card className="judex-auth-card">
      <Card.Header>
        <Card.Title>{t("loginTitle")}</Card.Title>
        <Card.Description>{t("loginHint")}</Card.Description>
      </Card.Header>
      <Card.Content>
        {location.state?.expired && (
          <UIWarning role="alert">{t("errSessionExpired")}</UIWarning>
        )}
        <form
          className="judex-auth-form"
          onSubmit={(event) => {
            event.preventDefault();
            mutation.mutate();
          }}
        >
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
            <Input autoComplete="current-password" />
          </TextField>
          <label className="judex-auth-reveal">
            <input
              type="checkbox"
              checked={reveal}
              onChange={(event) => setReveal(event.target.checked)}
            />
            {reveal ? t("hidePassword") : t("showPassword")}
          </label>
          <Button type="submit" isPending={mutation.isPending}>
            {mutation.isPending ? t("submitting") : t("submitLogin")}
          </Button>
          {mutation.isError && (
            <UIWarning role="alert" aria-live="polite">
              {t(errorKey(mutation.error) ?? "errNetwork")}
            </UIWarning>
          )}
        </form>
      </Card.Content>
      <Card.Footer>
        <Button variant="ghost" onClick={() => navigate("/register")}>
          {t("toRegister")}
        </Button>
        <Button variant="ghost" onClick={() => navigate("/recover")}>
          {t("toRecover")}
        </Button>
      </Card.Footer>
    </Card>
  );
}
