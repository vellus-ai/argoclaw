import { describe, expect, it } from "vitest";
import i18nInstance from "./index";
import { SUPPORTED_LANGUAGES } from "../lib/constants";

type JsonValue = string | number | boolean | null | JsonValue[] | { [k: string]: JsonValue };

const enModules = import.meta.glob("./locales/en/*.json", {
  eager: true,
  import: "default",
}) as Record<string, JsonValue>;

const allLocaleModules = import.meta.glob("./locales/*/*.json", {
  eager: true,
  import: "default",
}) as Record<string, JsonValue>;

function flattenKeys(value: JsonValue, prefix = ""): Set<string> {
  const keys = new Set<string>();
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    if (prefix) keys.add(prefix);
    return keys;
  }
  for (const [k, v] of Object.entries(value)) {
    const path = prefix ? `${prefix}.${k}` : k;
    if (v !== null && typeof v === "object" && !Array.isArray(v)) {
      for (const nested of flattenKeys(v, path)) keys.add(nested);
    } else {
      keys.add(path);
    }
  }
  return keys;
}

function flattenStrings(value: JsonValue, prefix = ""): Map<string, string> {
  const out = new Map<string, string>();
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    if (prefix && typeof value === "string") out.set(prefix, value);
    return out;
  }
  for (const [k, v] of Object.entries(value)) {
    const path = prefix ? `${prefix}.${k}` : k;
    if (v !== null && typeof v === "object" && !Array.isArray(v)) {
      for (const [nk, nv] of flattenStrings(v, path)) out.set(nk, nv);
    } else if (typeof v === "string") {
      out.set(path, v);
    }
  }
  return out;
}

function extractTokens(s: string): Set<string> {
  const tokens = new Set<string>();
  const re = /\{\{\s*([a-zA-Z0-9_]+)\s*\}\}/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(s)) !== null) {
    const name = m[1];
    if (name) tokens.add(name);
  }
  return tokens;
}

function namespaceFromPath(path: string): string {
  const m = path.match(/\/([^/]+)\.json$/);
  return m?.[1] ?? path;
}

function langFromPath(path: string): string {
  const m = path.match(/locales\/([^/]+)\//);
  return m?.[1] ?? "";
}

const EN_NAMESPACES = Object.keys(enModules).map(namespaceFromPath).sort();
const nonEnLangs = SUPPORTED_LANGUAGES.filter((l) => l !== "en");

describe("i18n completeness — file presence and key coverage", () => {
  it("EN provides at least one namespace", () => {
    expect(EN_NAMESPACES.length).toBeGreaterThan(0);
  });

  describe.each(nonEnLangs)("language: %s", (lang) => {
    const langModules: Record<string, JsonValue> = {};
    for (const [path, mod] of Object.entries(allLocaleModules)) {
      if (langFromPath(path) === lang) {
        langModules[namespaceFromPath(path)] = mod;
      }
    }

    it.each(EN_NAMESPACES)(`has namespace "%s"`, (ns) => {
      expect(langModules[ns], `missing file: locales/${lang}/${ns}.json`).toBeDefined();
    });

    it.each(EN_NAMESPACES)(`namespace "%s" contains every EN key`, (ns) => {
      const enFile = enModules[`./locales/en/${ns}.json`];
      const langFile = langModules[ns];
      if (enFile === undefined) {
        expect.fail(`missing file locales/en/${ns}.json`);
        return;
      }
      if (langFile === undefined) {
        expect.fail(`missing file locales/${lang}/${ns}.json`);
        return;
      }
      const enKeys = flattenKeys(enFile);
      const langKeys = flattenKeys(langFile);
      const missing: string[] = [];
      for (const k of enKeys) {
        if (!langKeys.has(k)) missing.push(k);
      }
      expect(
        missing,
        `locales/${lang}/${ns}.json is missing ${missing.length} keys: ${missing
          .slice(0, 5)
          .join(", ")}${missing.length > 5 ? ", ..." : ""}`,
      ).toEqual([]);
    });
  });
});

describe("i18n completeness — interpolation token parity", () => {
  describe.each(nonEnLangs)("language: %s", (lang) => {
    it.each(EN_NAMESPACES)(`namespace "%s" preserves every EN {{token}}`, (ns) => {
      const enFile = enModules[`./locales/en/${ns}.json`];
      const langFile = allLocaleModules[`./locales/${lang}/${ns}.json`];
      if (enFile === undefined || langFile === undefined) return;
      const enStrings = flattenStrings(enFile);
      const langStrings = flattenStrings(langFile);
      const broken: string[] = [];
      for (const [key, enVal] of enStrings) {
        const enTokens = extractTokens(enVal);
        if (enTokens.size === 0) continue;
        const langVal = langStrings.get(key);
        if (langVal === undefined) continue;
        const langTokens = extractTokens(langVal);
        for (const t of enTokens) {
          if (!langTokens.has(t)) broken.push(`${key} missing {{${t}}}`);
        }
      }
      expect(
        broken,
        `locales/${lang}/${ns}.json drops ${broken.length} interpolation token(s): ${broken
          .slice(0, 5)
          .join("; ")}${broken.length > 5 ? "; ..." : ""}`,
      ).toEqual([]);
    });
  });
});

describe("i18n completeness — runtime wiring via index.ts", () => {
  it.each(SUPPORTED_LANGUAGES)(`i18n instance registers every namespace for "%s"`, (lang) => {
    const missing: string[] = [];
    for (const ns of EN_NAMESPACES) {
      if (!i18nInstance.hasResourceBundle(lang, ns)) {
        missing.push(ns);
      }
    }
    expect(
      missing,
      `i18n resources.${lang} is missing ${missing.length} namespace(s): ${missing.join(", ")}`,
    ).toEqual([]);
  });
});
