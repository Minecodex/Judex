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
import { ArrowRight, Eye, EyeOff } from "lucide-react";
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
        <span className="judex-entry-badge">{t("portalSpace")}</span>
        <Card.Title>{t("portalWelcome")}</Card.Title>
        <Card.Description>{t("portalLoginHint")}</Card.Description>
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
            <Input autoComplete="email" placeholder={t("portalEmailPlaceholder")} />
          </TextField>
          <TextField
            isRequired
            name="password"
            type={reveal ? "text" : "password"}
            value={password}
            onChange={setPassword}
          >
            <div className="judex-field-heading"><Label>{t("fieldPassword")}</Label><Button size="sm" className="judex-text-link" onPress={() => navigate("/recover")}>{t("toRecover")}</Button></div>
            <div className="judex-password-field"><Input autoComplete="current-password" placeholder={t("portalPasswordPlaceholder")} /><Button isIconOnly size="sm" aria-label={reveal ? t("hidePassword") : t("showPassword")} onPress={() => setReveal(!reveal)}>{reveal ? <EyeOff /> : <Eye />}</Button></div>
          </TextField>
          <Button type="submit" variant="primary" size="lg" fullWidth isPending={mutation.isPending}>
            {mutation.isPending ? t("submitting") : t("submitLogin")}
            <ArrowRight />
          </Button>
          {mutation.isError && (
            <UIWarning role="alert" aria-live="polite">
              {t(errorKey(mutation.error) ?? "errNetwork")}
            </UIWarning>
          )}
        </form>
      </Card.Content>
      <Card.Footer>
        <span>{t("portalNewAccount")}</span>
        <Button variant="ghost" className="judex-text-link" onClick={() => navigate("/register")}>
          {t("portalCreateAccount")}
        </Button>
      </Card.Footer>
    </Card>
  );
}
