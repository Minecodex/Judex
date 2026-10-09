import { UINotice, UIWarning, UICheckbox } from "../../components/ui/FormControls";
import { Card, FieldError, Input, Label, TextField } from "@heroui/react";
import { Button } from "../../components/ui/Button";
import { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router";
import { useMutation } from "@tanstack/react-query";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { register } from "./api";
import { errorKey } from "./errors";
import { validateDisplayName, validateEmail, validatePassword } from "./validation";
import { safeReturnTo, useAuth } from "./AuthProvider";

type FieldErrors = { name?: Key | null; email?: Key | null; password?: Key | null; confirm?: Key | null };

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
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});

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

  const clear = (field: keyof FieldErrors) =>
    setFieldErrors((prev) => ({ ...prev, [field]: null }));

  const submit = () => {
    const errors: FieldErrors = {
      name: validateDisplayName(name),
      email: validateEmail(email.trim()),
      password: validatePassword(password),
      confirm: password !== confirmValue ? "errMismatch" : null,
    };
    if (errors.name || errors.email || errors.password || errors.confirm) {
      setFieldErrors(errors);
      return;
    }
    setFieldErrors({});
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
          <TextField
            isRequired
            name="displayName"
            value={name}
            onChange={(v) => { setName(v); clear("name"); }}
            isInvalid={!!fieldErrors.name}
          >
            <Label>{t("fieldDisplayName")}</Label>
            <Input autoComplete="name" />
            <FieldError>{fieldErrors.name ? t(fieldErrors.name) : ""}</FieldError>
          </TextField>
          <TextField
            isRequired
            type="email"
            name="email"
            value={email}
            onChange={(v) => { setEmail(v); clear("email"); }}
            isInvalid={!!fieldErrors.email}
          >
            <Label>{t("fieldEmail")}</Label>
            <Input autoComplete="email" />
            <FieldError>{fieldErrors.email ? t(fieldErrors.email) : ""}</FieldError>
          </TextField>
          <TextField
            isRequired
            name="password"
            type={reveal ? "text" : "password"}
            value={password}
            onChange={(v) => { setPassword(v); clear("password"); }}
            isInvalid={!!fieldErrors.password}
          >
            <Label>{t("fieldPassword")}</Label>
            <Input autoComplete="new-password" />
            <FieldError>{fieldErrors.password ? t(fieldErrors.password) : ""}</FieldError>
          </TextField>
          <TextField
            isRequired
            name="passwordConfirm"
            type={reveal ? "text" : "password"}
            value={confirmValue}
            onChange={(v) => { setConfirmValue(v); clear("confirm"); }}
            isInvalid={!!fieldErrors.confirm}
          >
            <Label>{t("fieldPasswordConfirm")}</Label>
            <Input autoComplete="new-password" />
            <FieldError>{fieldErrors.confirm ? t(fieldErrors.confirm) : ""}</FieldError>
          </TextField>
          <UICheckbox checked={reveal} onChange={() => setReveal(!reveal)}>{reveal ? t("hidePassword") : t("showPassword")}</UICheckbox>
          <small className="judex-auth-hint" aria-live="polite">{t("passwordRule")}</small>
          <Button variant="primary" fullWidth size="lg" type="submit" isPending={mutation.isPending}>
            {mutation.isPending ? t("submitting") : t("submitRegister")}
          </Button>
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
