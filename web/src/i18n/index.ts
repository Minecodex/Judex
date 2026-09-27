import { commonZh, commonEn } from "./common";
import { experienceZh, experienceEn } from "./experience";
import { workflowZh, workflowEn } from "./workflow";
import { agentsZh, agentsEn } from "./agents";
import { membershipZh, membershipEn } from "./membership";
import { workZh, workEn } from "./work";
import { projectZh, projectEn } from "./project";
import { cardsZh, cardsEn } from "./cards";
import { shellZh, shellEn } from "./shell";
import { chatZh, chatEn } from "./chat";
import { settingsZh, settingsEn } from "./settings";
const modules = [
  { zh: commonZh, en: commonEn },
  { zh: experienceZh, en: experienceEn },
  { zh: workflowZh, en: workflowEn },
  { zh: agentsZh, en: agentsEn },
  { zh: membershipZh, en: membershipEn },
  { zh: workZh, en: workEn },
  { zh: projectZh, en: projectEn },
  { zh: cardsZh, en: cardsEn },
  { zh: shellZh, en: shellEn },
  { zh: chatZh, en: chatEn },
  { zh: settingsZh, en: settingsEn },
];
export type Locale = "zh-CN" | "en";
export type Key =
  | keyof typeof commonZh
  | keyof typeof experienceZh
  | keyof typeof workflowZh
  | keyof typeof agentsZh
  | keyof typeof membershipZh
  | keyof typeof workZh
  | keyof typeof projectZh
  | keyof typeof cardsZh
  | keyof typeof shellZh
  | keyof typeof chatZh
  | keyof typeof settingsZh;
const zh = Object.assign({}, ...modules.map((m) => m.zh)) as Record<
  Key,
  string
>;
const en = Object.assign({}, ...modules.map((m) => m.en)) as Record<
  Key,
  string
>;
export const translate = (
  locale: Locale,
  key: Key,
  values: Record<string, string | number> = {},
) => {
  let value = (locale === "zh-CN" ? zh : en)[key];
  Object.entries(values).forEach(([k, v]) => {
    value = value.replaceAll("{" + k + "}", String(v));
  });
  return value;
};
