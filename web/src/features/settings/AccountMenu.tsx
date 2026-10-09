import {
  UIOption,
  UISelect,
  UIDisclosure,
  UIWarning,
} from "../../components/ui/FormControls";
import { useState } from "react";
import { Popover } from "@heroui/react";
import { Button } from "../../components/ui/Button";
import {
  ChevronUp,
  UserPlus,
  Settings2,
  LogOut,
  Globe,
  Sun,
  Moon,
  FolderOpen,
} from "lucide-react";
import { useWork } from "../work/store";
import { InviteDialog } from "../work/TeamPages";
import { dataMode } from "../../lib/api/client";
import { PersonAvatar } from "../../components/ui/Presentation";
export function AccountMenu({ onLogout, onBackToProjects }: {
  onLogout: () => Promise<void>;
  onBackToProjects?: () => void;
}) {
  const {
    state,
    t,
    locale,
    setLocale,
    theme,
    setTheme,
    people,
    switchPerson,
    management,
    project,
    go,
  } = useWork();
  const [open, setOpen] = useState(false),
    [invite, setInvite] = useState(false),
    [pending, setPending] = useState(false),
    [error, setError] = useState("");
  return (
    <>
      <Popover isOpen={open} onOpenChange={setOpen}>
        <Button
          variant="ghost"
          className="judex-account-trigger"
          data-testid="account-menu-button"
          aria-label={t("accountMenu")}
        >
          <PersonAvatar name={state.currentUser} />
          <span>
            <strong>{state.currentUser}</strong>
            {dataMode === "demo" && <small>{t("chatDemoPerson")}</small>}
          </span>
          <ChevronUp size={16} />
        </Button>
        <Popover.Content
          placement="top start"
          className="judex-account-popover"
        >
          <Popover.Dialog
            className="judex-account-menu"
            aria-label={t("accountMenu")}
          >
            <div className="judex-account-menu-identity">
              <span className="judex-chat-avatar">
                {state.currentUser.slice(0, 1)}
              </span>
              <div>
                <strong>{state.currentUser}</strong>
                <small>{project.title[locale === "en" ? "en" : "zh"]}</small>
              </div>
            </div>
            <Button
              variant="ghost"
              className="judex-account-menu-action"
              data-testid="account-invite"
              isDisabled={!management}
              onPress={() => {
                setOpen(false);
                setInvite(true);
              }}
            >
              <UserPlus size={17} />
              {t("accountInvite")}
            </Button>
            {!management && (
              <small className="judex-account-permission">
                {t("accountInviteScope")}
              </small>
            )}
            <Button
              variant="ghost"
              className="judex-account-menu-action"
              data-testid="account-settings"
              onPress={() => {
                setOpen(false);
                go({
                  settingsSection: "general",
                  settingsItem: undefined,
                });
              }}
            >
              <Settings2 size={17} />
              {t("accountSettings")}
            </Button>
            {onBackToProjects && (
              <Button
                variant="ghost"
                className="judex-account-menu-action"
                data-testid="account-projects"
                onPress={() => {
                  setOpen(false);
                  onBackToProjects();
                }}
              >
                <FolderOpen size={17} />
                {t("accountProjects")}
              </Button>
            )}
            <div className="judex-account-menu-divider" />
            <Button
              variant="ghost"
              className="judex-account-menu-action"
              data-testid="next-language"
              onPress={() => setLocale(locale === "en" ? "zh-CN" : "en")}
            >
              <Globe size={17} />
              <span>{t("accountLanguage")}</span>
              <small>{locale === "en" ? "English" : "简体中文"}</small>
            </Button>
            <Button
              variant="ghost"
              className="judex-account-menu-action"
              data-testid="next-theme"
              onPress={() => setTheme(theme === "dark" ? "light" : "dark")}
            >
              {theme === "dark" ? <Moon size={17} /> : <Sun size={17} />}
              <span>{t("accountTheme")}</span>
              <small>
                {t(theme === "dark" ? "accountDark" : "accountLight")}
              </small>
            </Button>
            {dataMode === "demo" && (
              <UIDisclosure
                className="judex-account-preview"
                title={<>{t("chatDemoPerson")}</>}
              >
                <p>{t("accountPreviewNote")}</p>
                <UISelect
                  data-testid="work-person"
                  aria-label={t("chatDemoPerson")}
                  value={state.currentUser}
                  onChange={(e) => {
                    switchPerson(e.target.value);
                    setOpen(false);
                  }}
                >
                  {people.map((person) => (
                    <UIOption key={person}>{person}</UIOption>
                  ))}
                </UISelect>
              </UIDisclosure>
            )}
            <div className="judex-account-menu-divider" />
            <Button
              variant="ghost"
              className="judex-account-menu-action"
              data-testid="account-logout"
              isPending={pending}
              onPress={async () => {
                setPending(true);
                setError("");
                try {
                  await onLogout();
                } catch {
                  setPending(false);
                  setError(t("accountLogoutFailed"));
                }
              }}
            >
              <LogOut size={17} />
              {t("accountLogout")}
            </Button>
            {error && <UIWarning role="alert">{error}</UIWarning>}
          </Popover.Dialog>
        </Popover.Content>
      </Popover>
      {invite && <InviteDialog onClose={() => setInvite(false)} />}
    </>
  );
}
