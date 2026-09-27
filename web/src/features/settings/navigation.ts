import type { View } from "../work/types.ts";
export type SettingsSection =
  "general" | "preferences" | "project" | "team" | "flows" | "local";
export const settingsSections: SettingsSection[] = [
  "general",
  "preferences",
  "project",
  "team",
  "flows",
  "local",
];
export function settingsDestination(
  view?: View,
  id?: string,
): { section: SettingsSection; item?: string } | null {
  if (view === "team") return { section: "team" };
  if (view === "flows") return { section: "flows", item: id };
  if (view === "settings")
    return {
      section:
        id === "project" ? "project" : id === "local" ? "local" : "preferences",
    };
  return null;
}
