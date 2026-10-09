import {materialsZh,materialsEn} from "./materials";
import {materialPreviewZh,materialPreviewEn} from "./materialPreview";
import {cooperationZh,cooperationEn} from "./cooperation";
import {taskRoutePreviewZh,taskRoutePreviewEn} from "./taskRoutePreview";
import {taskDetailsZh,taskDetailsEn} from './taskDetails';
import {collaborationZh,collaborationEn} from "./collaboration";
import {cooperationPreviewZh,cooperationPreviewEn} from "./cooperationPreview";
import { accessZh, accessEn } from "./access";
import { portalZh, portalEn } from "./portal";
import { connectionZh, connectionEn } from "./connection";
import { positionPresetsZh, positionPresetsEn } from "./positionPresets";
import {workflowPresetsZh,workflowPresetsEn} from './workflowPresets';
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
import { authZh, authEn } from "./auth";
import { lifecycleZh, lifecycleEn } from "./lifecycle";
const modules = [
 {zh:taskDetailsZh,en:taskDetailsEn},
 {zh:materialsZh,en:materialsEn},
 {zh:taskRoutePreviewZh,en:taskRoutePreviewEn},
 {zh:materialPreviewZh,en:materialPreviewEn},
 {zh:cooperationZh,en:cooperationEn},
 {zh:cooperationPreviewZh,en:cooperationPreviewEn},
 {zh:collaborationZh,en:collaborationEn},
  {zh:workflowPresetsZh,en:workflowPresetsEn},
  { zh: positionPresetsZh, en: positionPresetsEn },
  { zh: connectionZh, en: connectionEn },
  { zh: portalZh, en: portalEn },
  { zh: accessZh, en: accessEn },
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
  { zh: authZh, en: authEn },
  { zh: lifecycleZh, en: lifecycleEn },
];
export type Locale = "zh-CN" | "en";
// re-exported for feature modules
export type Key =
 | keyof typeof taskDetailsZh
 | keyof typeof materialsZh
 | keyof typeof taskRoutePreviewZh
 | keyof typeof materialPreviewZh
 | keyof typeof cooperationZh
 | keyof typeof cooperationPreviewZh
 | keyof typeof collaborationZh
  | keyof typeof workflowPresetsZh
  | keyof typeof positionPresetsZh
  | keyof typeof connectionZh
  | keyof typeof portalZh
  | keyof typeof accessZh
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
  | keyof typeof settingsZh
  | keyof typeof authZh
  | keyof typeof lifecycleZh;
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
