import i18n from "i18next";
import { initReactI18next } from "react-i18next";

// --- EN namespaces ---
import enCommon from "./locales/en/common.json";
import enSidebar from "./locales/en/sidebar.json";
import enTopbar from "./locales/en/topbar.json";
import enLogin from "./locales/en/login.json";
import enOverview from "./locales/en/overview.json";
import enChat from "./locales/en/chat.json";
import enAgents from "./locales/en/agents.json";
import enTeams from "./locales/en/teams.json";
import enSessions from "./locales/en/sessions.json";
import enSkills from "./locales/en/skills.json";
import enCron from "./locales/en/cron.json";
import enConfig from "./locales/en/config.json";
import enChannels from "./locales/en/channels.json";
import enProviders from "./locales/en/providers.json";
import enTraces from "./locales/en/traces.json";
import enEvents from "./locales/en/events.json";
import enUsage from "./locales/en/usage.json";
import enApprovals from "./locales/en/approvals.json";
import enNodes from "./locales/en/nodes.json";
import enLogs from "./locales/en/logs.json";
import enTools from "./locales/en/tools.json";
import enMcp from "./locales/en/mcp.json";
import enTts from "./locales/en/tts.json";
import enSetup from "./locales/en/setup.json";
import enMemory from "./locales/en/memory.json";
import enStorage from "./locales/en/storage.json";
import enPendingMessages from "./locales/en/pending-messages.json";
import enContacts from "./locales/en/contacts.json";
import enActivity from "./locales/en/activity.json";
import enApiKeys from "./locales/en/api-keys.json";
import enCliCredentials from "./locales/en/cli-credentials.json";
import enPackages from "./locales/en/packages.json";
import enProjects from "./locales/en/projects.json";

// --- VI namespaces ---
import viCommon from "./locales/vi/common.json";
import viSidebar from "./locales/vi/sidebar.json";
import viTopbar from "./locales/vi/topbar.json";
import viLogin from "./locales/vi/login.json";
import viOverview from "./locales/vi/overview.json";
import viChat from "./locales/vi/chat.json";
import viAgents from "./locales/vi/agents.json";
import viTeams from "./locales/vi/teams.json";
import viSessions from "./locales/vi/sessions.json";
import viSkills from "./locales/vi/skills.json";
import viCron from "./locales/vi/cron.json";
import viConfig from "./locales/vi/config.json";
import viChannels from "./locales/vi/channels.json";
import viProviders from "./locales/vi/providers.json";
import viTraces from "./locales/vi/traces.json";
import viEvents from "./locales/vi/events.json";
import viUsage from "./locales/vi/usage.json";
import viApprovals from "./locales/vi/approvals.json";
import viNodes from "./locales/vi/nodes.json";
import viLogs from "./locales/vi/logs.json";
import viTools from "./locales/vi/tools.json";
import viMcp from "./locales/vi/mcp.json";
import viTts from "./locales/vi/tts.json";
import viSetup from "./locales/vi/setup.json";
import viMemory from "./locales/vi/memory.json";
import viStorage from "./locales/vi/storage.json";
import viPendingMessages from "./locales/vi/pending-messages.json";
import viContacts from "./locales/vi/contacts.json";
import viActivity from "./locales/vi/activity.json";
import viApiKeys from "./locales/vi/api-keys.json";
import viCliCredentials from "./locales/vi/cli-credentials.json";
import viPackages from "./locales/vi/packages.json";
import viProjects from "./locales/vi/projects.json";

// --- PT namespaces ---
import ptCommon from "./locales/pt/common.json";
import ptSidebar from "./locales/pt/sidebar.json";
import ptTopbar from "./locales/pt/topbar.json";
import ptLogin from "./locales/pt/login.json";
import ptOverview from "./locales/pt/overview.json";
import ptChat from "./locales/pt/chat.json";
import ptAgents from "./locales/pt/agents.json";
import ptTeams from "./locales/pt/teams.json";
import ptSessions from "./locales/pt/sessions.json";
import ptSkills from "./locales/pt/skills.json";
import ptCron from "./locales/pt/cron.json";
import ptConfig from "./locales/pt/config.json";
import ptChannels from "./locales/pt/channels.json";
import ptProviders from "./locales/pt/providers.json";
import ptTraces from "./locales/pt/traces.json";
import ptEvents from "./locales/pt/events.json";
import ptUsage from "./locales/pt/usage.json";
import ptApprovals from "./locales/pt/approvals.json";
import ptNodes from "./locales/pt/nodes.json";
import ptLogs from "./locales/pt/logs.json";
import ptTools from "./locales/pt/tools.json";
import ptMcp from "./locales/pt/mcp.json";
import ptTts from "./locales/pt/tts.json";
import ptSetup from "./locales/pt/setup.json";
import ptMemory from "./locales/pt/memory.json";
import ptStorage from "./locales/pt/storage.json";
import ptPendingMessages from "./locales/pt/pending-messages.json";
import ptContacts from "./locales/pt/contacts.json";
import ptActivity from "./locales/pt/activity.json";
import ptApiKeys from "./locales/pt/api-keys.json";
import ptCliCredentials from "./locales/pt/cli-credentials.json";
import ptPackages from "./locales/pt/packages.json";
import ptProjects from "./locales/pt/projects.json";

// --- ES namespaces ---
import esCommon from "./locales/es/common.json";
import esSidebar from "./locales/es/sidebar.json";
import esTopbar from "./locales/es/topbar.json";
import esLogin from "./locales/es/login.json";
import esOverview from "./locales/es/overview.json";
import esChat from "./locales/es/chat.json";
import esAgents from "./locales/es/agents.json";
import esTeams from "./locales/es/teams.json";
import esSessions from "./locales/es/sessions.json";
import esSkills from "./locales/es/skills.json";
import esCron from "./locales/es/cron.json";
import esConfig from "./locales/es/config.json";
import esChannels from "./locales/es/channels.json";
import esProviders from "./locales/es/providers.json";
import esTraces from "./locales/es/traces.json";
import esEvents from "./locales/es/events.json";
import esUsage from "./locales/es/usage.json";
import esApprovals from "./locales/es/approvals.json";
import esNodes from "./locales/es/nodes.json";
import esLogs from "./locales/es/logs.json";
import esTools from "./locales/es/tools.json";
import esMcp from "./locales/es/mcp.json";
import esTts from "./locales/es/tts.json";
import esSetup from "./locales/es/setup.json";
import esMemory from "./locales/es/memory.json";
import esStorage from "./locales/es/storage.json";
import esPendingMessages from "./locales/es/pending-messages.json";
import esContacts from "./locales/es/contacts.json";
import esActivity from "./locales/es/activity.json";
import esApiKeys from "./locales/es/api-keys.json";
import esCliCredentials from "./locales/es/cli-credentials.json";
import esPackages from "./locales/es/packages.json";
import esProjects from "./locales/es/projects.json";

// --- FR namespaces ---
import frCommon from "./locales/fr/common.json";
import frSidebar from "./locales/fr/sidebar.json";
import frTopbar from "./locales/fr/topbar.json";
import frLogin from "./locales/fr/login.json";
import frOverview from "./locales/fr/overview.json";
import frChat from "./locales/fr/chat.json";
import frAgents from "./locales/fr/agents.json";
import frTeams from "./locales/fr/teams.json";
import frSessions from "./locales/fr/sessions.json";
import frSkills from "./locales/fr/skills.json";
import frCron from "./locales/fr/cron.json";
import frConfig from "./locales/fr/config.json";
import frChannels from "./locales/fr/channels.json";
import frProviders from "./locales/fr/providers.json";
import frTraces from "./locales/fr/traces.json";
import frEvents from "./locales/fr/events.json";
import frUsage from "./locales/fr/usage.json";
import frApprovals from "./locales/fr/approvals.json";
import frNodes from "./locales/fr/nodes.json";
import frLogs from "./locales/fr/logs.json";
import frTools from "./locales/fr/tools.json";
import frMcp from "./locales/fr/mcp.json";
import frTts from "./locales/fr/tts.json";
import frSetup from "./locales/fr/setup.json";
import frMemory from "./locales/fr/memory.json";
import frStorage from "./locales/fr/storage.json";
import frPendingMessages from "./locales/fr/pending-messages.json";
import frContacts from "./locales/fr/contacts.json";
import frActivity from "./locales/fr/activity.json";
import frApiKeys from "./locales/fr/api-keys.json";
import frCliCredentials from "./locales/fr/cli-credentials.json";
import frPackages from "./locales/fr/packages.json";
import frProjects from "./locales/fr/projects.json";

// --- IT namespaces ---
import itCommon from "./locales/it/common.json";
import itSidebar from "./locales/it/sidebar.json";
import itTopbar from "./locales/it/topbar.json";
import itLogin from "./locales/it/login.json";
import itOverview from "./locales/it/overview.json";
import itChat from "./locales/it/chat.json";
import itAgents from "./locales/it/agents.json";
import itTeams from "./locales/it/teams.json";
import itSessions from "./locales/it/sessions.json";
import itSkills from "./locales/it/skills.json";
import itCron from "./locales/it/cron.json";
import itConfig from "./locales/it/config.json";
import itChannels from "./locales/it/channels.json";
import itProviders from "./locales/it/providers.json";
import itTraces from "./locales/it/traces.json";
import itEvents from "./locales/it/events.json";
import itUsage from "./locales/it/usage.json";
import itApprovals from "./locales/it/approvals.json";
import itNodes from "./locales/it/nodes.json";
import itLogs from "./locales/it/logs.json";
import itTools from "./locales/it/tools.json";
import itMcp from "./locales/it/mcp.json";
import itTts from "./locales/it/tts.json";
import itSetup from "./locales/it/setup.json";
import itMemory from "./locales/it/memory.json";
import itStorage from "./locales/it/storage.json";
import itPendingMessages from "./locales/it/pending-messages.json";
import itContacts from "./locales/it/contacts.json";
import itActivity from "./locales/it/activity.json";
import itApiKeys from "./locales/it/api-keys.json";
import itCliCredentials from "./locales/it/cli-credentials.json";
import itPackages from "./locales/it/packages.json";
import itProjects from "./locales/it/projects.json";

// --- DE namespaces ---
import deCommon from "./locales/de/common.json";
import deSidebar from "./locales/de/sidebar.json";
import deTopbar from "./locales/de/topbar.json";
import deLogin from "./locales/de/login.json";
import deOverview from "./locales/de/overview.json";
import deChat from "./locales/de/chat.json";
import deAgents from "./locales/de/agents.json";
import deTeams from "./locales/de/teams.json";
import deSessions from "./locales/de/sessions.json";
import deSkills from "./locales/de/skills.json";
import deCron from "./locales/de/cron.json";
import deConfig from "./locales/de/config.json";
import deChannels from "./locales/de/channels.json";
import deProviders from "./locales/de/providers.json";
import deTraces from "./locales/de/traces.json";
import deEvents from "./locales/de/events.json";
import deUsage from "./locales/de/usage.json";
import deApprovals from "./locales/de/approvals.json";
import deNodes from "./locales/de/nodes.json";
import deLogs from "./locales/de/logs.json";
import deTools from "./locales/de/tools.json";
import deMcp from "./locales/de/mcp.json";
import deTts from "./locales/de/tts.json";
import deSetup from "./locales/de/setup.json";
import deMemory from "./locales/de/memory.json";
import deStorage from "./locales/de/storage.json";
import dePendingMessages from "./locales/de/pending-messages.json";
import deContacts from "./locales/de/contacts.json";
import deActivity from "./locales/de/activity.json";
import deApiKeys from "./locales/de/api-keys.json";
import deCliCredentials from "./locales/de/cli-credentials.json";
import dePackages from "./locales/de/packages.json";
import deProjects from "./locales/de/projects.json";

// --- ZH namespaces ---
import zhCommon from "./locales/zh/common.json";
import zhSidebar from "./locales/zh/sidebar.json";
import zhTopbar from "./locales/zh/topbar.json";
import zhLogin from "./locales/zh/login.json";
import zhOverview from "./locales/zh/overview.json";
import zhChat from "./locales/zh/chat.json";
import zhAgents from "./locales/zh/agents.json";
import zhTeams from "./locales/zh/teams.json";
import zhSessions from "./locales/zh/sessions.json";
import zhSkills from "./locales/zh/skills.json";
import zhCron from "./locales/zh/cron.json";
import zhConfig from "./locales/zh/config.json";
import zhChannels from "./locales/zh/channels.json";
import zhProviders from "./locales/zh/providers.json";
import zhTraces from "./locales/zh/traces.json";
import zhEvents from "./locales/zh/events.json";
import zhUsage from "./locales/zh/usage.json";
import zhApprovals from "./locales/zh/approvals.json";
import zhNodes from "./locales/zh/nodes.json";
import zhLogs from "./locales/zh/logs.json";
import zhTools from "./locales/zh/tools.json";
import zhMcp from "./locales/zh/mcp.json";
import zhTts from "./locales/zh/tts.json";
import zhSetup from "./locales/zh/setup.json";
import zhMemory from "./locales/zh/memory.json";
import zhStorage from "./locales/zh/storage.json";
import zhPendingMessages from "./locales/zh/pending-messages.json";
import zhContacts from "./locales/zh/contacts.json";
import zhActivity from "./locales/zh/activity.json";
import zhApiKeys from "./locales/zh/api-keys.json";
import zhCliCredentials from "./locales/zh/cli-credentials.json";
import zhPackages from "./locales/zh/packages.json";
import zhProjects from "./locales/zh/projects.json";

import { SUPPORTED_LANGUAGES } from "../lib/constants";

const STORAGE_KEY = "argo:language";

type SupportedLang = (typeof SUPPORTED_LANGUAGES)[number];

function getInitialLanguage(): SupportedLang {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored && (SUPPORTED_LANGUAGES as readonly string[]).includes(stored)) return stored as SupportedLang;
  const lang = navigator.language.toLowerCase();
  // Match browser language prefix to supported languages
  for (const supported of SUPPORTED_LANGUAGES) {
    if (lang.startsWith(supported)) return supported;
  }
  return "en";
}

const ns = [
  "common", "sidebar", "topbar", "login", "overview", "chat",
  "agents", "teams", "sessions", "skills", "cron", "config",
  "channels", "providers", "traces", "events",
  "usage", "approvals", "nodes", "logs", "tools", "mcp", "tts",
  "setup", "memory", "storage", "pending-messages", "contacts", "activity", "api-keys",
  "cli-credentials", "packages", "projects",
] as const;

i18n.use(initReactI18next).init({
  resources: {
    en: {
      common: enCommon, sidebar: enSidebar, topbar: enTopbar, login: enLogin,
      overview: enOverview, chat: enChat, agents: enAgents, teams: enTeams,
      sessions: enSessions, skills: enSkills, cron: enCron, config: enConfig,
      channels: enChannels, providers: enProviders, traces: enTraces,
      events: enEvents, usage: enUsage,
      approvals: enApprovals, nodes: enNodes, logs: enLogs, tools: enTools,
      mcp: enMcp, tts: enTts, setup: enSetup, memory: enMemory, storage: enStorage,
      "pending-messages": enPendingMessages,
      contacts: enContacts, activity: enActivity, "api-keys": enApiKeys,
      "cli-credentials": enCliCredentials,
      packages: enPackages, projects: enProjects,
    },
    vi: {
      common: viCommon, sidebar: viSidebar, topbar: viTopbar, login: viLogin,
      overview: viOverview, chat: viChat, agents: viAgents, teams: viTeams,
      sessions: viSessions, skills: viSkills, cron: viCron, config: viConfig,
      channels: viChannels, providers: viProviders, traces: viTraces,
      events: viEvents, usage: viUsage,
      approvals: viApprovals, nodes: viNodes, logs: viLogs, tools: viTools,
      mcp: viMcp, tts: viTts, setup: viSetup, memory: viMemory, storage: viStorage,
      "pending-messages": viPendingMessages,
      contacts: viContacts, activity: viActivity, "api-keys": viApiKeys,
      "cli-credentials": viCliCredentials,
      packages: viPackages, projects: viProjects,
    },
    zh: {
      common: zhCommon, sidebar: zhSidebar, topbar: zhTopbar, login: zhLogin,
      overview: zhOverview, chat: zhChat, agents: zhAgents, teams: zhTeams,
      sessions: zhSessions, skills: zhSkills, cron: zhCron, config: zhConfig,
      channels: zhChannels, providers: zhProviders, traces: zhTraces,
      events: zhEvents, usage: zhUsage,
      approvals: zhApprovals, nodes: zhNodes, logs: zhLogs, tools: zhTools,
      mcp: zhMcp, tts: zhTts, setup: zhSetup, memory: zhMemory, storage: zhStorage,
      "pending-messages": zhPendingMessages,
      contacts: zhContacts, activity: zhActivity, "api-keys": zhApiKeys,
      "cli-credentials": zhCliCredentials,
      packages: zhPackages, projects: zhProjects,
    },
    pt: {
      common: ptCommon, sidebar: ptSidebar, topbar: ptTopbar, login: ptLogin,
      overview: ptOverview, chat: ptChat, agents: ptAgents, teams: ptTeams,
      sessions: ptSessions, skills: ptSkills, cron: ptCron, config: ptConfig,
      channels: ptChannels, providers: ptProviders, traces: ptTraces,
      events: ptEvents, usage: ptUsage,
      approvals: ptApprovals, nodes: ptNodes, logs: ptLogs, tools: ptTools,
      mcp: ptMcp, tts: ptTts, setup: ptSetup, memory: ptMemory, storage: ptStorage,
      "pending-messages": ptPendingMessages,
      contacts: ptContacts, activity: ptActivity, "api-keys": ptApiKeys,
      "cli-credentials": ptCliCredentials,
      packages: ptPackages, projects: ptProjects,
    },
    es: {
      common: esCommon, sidebar: esSidebar, topbar: esTopbar, login: esLogin,
      overview: esOverview, chat: esChat, agents: esAgents, teams: esTeams,
      sessions: esSessions, skills: esSkills, cron: esCron, config: esConfig,
      channels: esChannels, providers: esProviders, traces: esTraces,
      events: esEvents, usage: esUsage,
      approvals: esApprovals, nodes: esNodes, logs: esLogs, tools: esTools,
      mcp: esMcp, tts: esTts, setup: esSetup, memory: esMemory, storage: esStorage,
      "pending-messages": esPendingMessages,
      contacts: esContacts, activity: esActivity, "api-keys": esApiKeys,
      "cli-credentials": esCliCredentials,
      packages: esPackages, projects: esProjects,
    },
    fr: {
      common: frCommon, sidebar: frSidebar, topbar: frTopbar, login: frLogin,
      overview: frOverview, chat: frChat, agents: frAgents, teams: frTeams,
      sessions: frSessions, skills: frSkills, cron: frCron, config: frConfig,
      channels: frChannels, providers: frProviders, traces: frTraces,
      events: frEvents, usage: frUsage,
      approvals: frApprovals, nodes: frNodes, logs: frLogs, tools: frTools,
      mcp: frMcp, tts: frTts, setup: frSetup, memory: frMemory, storage: frStorage,
      "pending-messages": frPendingMessages,
      contacts: frContacts, activity: frActivity, "api-keys": frApiKeys,
      "cli-credentials": frCliCredentials,
      packages: frPackages, projects: frProjects,
    },
    it: {
      common: itCommon, sidebar: itSidebar, topbar: itTopbar, login: itLogin,
      overview: itOverview, chat: itChat, agents: itAgents, teams: itTeams,
      sessions: itSessions, skills: itSkills, cron: itCron, config: itConfig,
      channels: itChannels, providers: itProviders, traces: itTraces,
      events: itEvents, usage: itUsage,
      approvals: itApprovals, nodes: itNodes, logs: itLogs, tools: itTools,
      mcp: itMcp, tts: itTts, setup: itSetup, memory: itMemory, storage: itStorage,
      "pending-messages": itPendingMessages,
      contacts: itContacts, activity: itActivity, "api-keys": itApiKeys,
      "cli-credentials": itCliCredentials,
      packages: itPackages, projects: itProjects,
    },
    de: {
      common: deCommon, sidebar: deSidebar, topbar: deTopbar, login: deLogin,
      overview: deOverview, chat: deChat, agents: deAgents, teams: deTeams,
      sessions: deSessions, skills: deSkills, cron: deCron, config: deConfig,
      channels: deChannels, providers: deProviders, traces: deTraces,
      events: deEvents, usage: deUsage,
      approvals: deApprovals, nodes: deNodes, logs: deLogs, tools: deTools,
      mcp: deMcp, tts: deTts, setup: deSetup, memory: deMemory, storage: deStorage,
      "pending-messages": dePendingMessages,
      contacts: deContacts, activity: deActivity, "api-keys": deApiKeys,
      "cli-credentials": deCliCredentials,
      packages: dePackages, projects: deProjects,
    },
  },
  ns: [...ns],
  defaultNS: "common",
  lng: getInitialLanguage(),
  fallbackLng: "en",
  interpolation: { escapeValue: false },
  missingKeyHandler: import.meta.env.DEV
    ? (_lngs, _ns, key) => console.warn(`[i18n] missing: ${key}`)
    : undefined,
});

i18n.on("languageChanged", (lng) => {
  localStorage.setItem(STORAGE_KEY, lng);
  document.documentElement.lang = lng;
});

export default i18n;
