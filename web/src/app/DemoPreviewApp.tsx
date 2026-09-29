import { Suspense, lazy, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Button, Card } from "@heroui/react";
import { usePreferences } from "../stores/preferences";
import { translate, type Key } from "../i18n";
const WorkApp = lazy(() => import("../features/chat/ChatWorkspace"));
// 演示模式的退出预览态：无会话概念，退出只进入引导页并可返回，
// 刷新不自动重入（sessionSession judex.preview.signedOut 持久）。
const SIGNED_OUT_KEY = "judex.preview.signedOut";

export default function DemoPreviewApp() {
  const [signedOut, setSignedOut] = useState(
    () => sessionStorage.getItem(SIGNED_OUT_KEY) === "1",
  );
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const client = useQueryClient();

  if (signedOut)
    return (
      <main className="judex-entry min-h-screen flex items-center justify-center">
        <Card className="judex-entry-card">
          <Card.Header>
            <Card.Title>{t("accountSignedOut")}</Card.Title>
            <Card.Description>{t("accountSignedOutHint")}</Card.Description>
          </Card.Header>
          <Card.Content>
            <Button
              data-testid="return-preview"
              onPress={() => {
                sessionStorage.removeItem(SIGNED_OUT_KEY);
                history.replaceState(null, "", "/");
                setSignedOut(false);
              }}
            >
              {t("accountReturnPreview")}
            </Button>
          </Card.Content>
        </Card>
      </main>
    );
  return (
    <Suspense fallback={<main className="judex-entry min-h-screen" />}>
      <WorkApp
        onLogout={async () => {
          sessionStorage.setItem(SIGNED_OUT_KEY, "1");
          await client.cancelQueries();
          client.clear();
          history.replaceState(null, "", "/");
          setSignedOut(true);
        }}
      />
    </Suspense>
  );
}

