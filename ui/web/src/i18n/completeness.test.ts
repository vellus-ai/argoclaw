import { describe, expect, it } from "vitest";
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

function namespaceFromPath(path: string): string {
  const m = path.match(/\/([^/]+)\.json$/);
  return m?.[1] ?? path;
}

function langFromPath(path: string): string {
  const m = path.match(/locales\/([^/]+)\//);
  return m?.[1] ?? "";
}

const EN_NAMESPACES = Object.keys(enModules).map(namespaceFromPath).sort();

describe("i18n completeness — Property 2", () => {
  it("EN provides all 33 namespaces expected by the platform", () => {
    expect(EN_NAMESPACES.length).toBe(33);
  });

  it("covers 8 supported languages × 33 namespaces = 264 combinations", () => {
    expect(SUPPORTED_LANGUAGES.length * EN_NAMESPACES.length).toBe(264);
  });

  const nonEnLangs = SUPPORTED_LANGUAGES.filter((l) => l !== "en");

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
