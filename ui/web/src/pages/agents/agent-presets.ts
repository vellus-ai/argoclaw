import { useTranslation } from "react-i18next";

export interface AgentPreset {
  label: string;
  prompt: string;
}

export function useAgentPresets(): AgentPreset[] {
  const { t } = useTranslation("agents");
  return [
    {
      label: t("presets.captain.label"),
      prompt: t("presets.captain.prompt"),
    },
    {
      label: t("presets.helmsman.label"),
      prompt: t("presets.helmsman.prompt"),
    },
    {
      label: t("presets.lookout.label"),
      prompt: t("presets.lookout.prompt"),
    },
    {
      label: t("presets.gunner.label"),
      prompt: t("presets.gunner.prompt"),
    },
    {
      label: t("presets.navigator.label"),
      prompt: t("presets.navigator.prompt"),
    },
    {
      label: t("presets.smith.label"),
      prompt: t("presets.smith.prompt"),
    },
  ];
}
