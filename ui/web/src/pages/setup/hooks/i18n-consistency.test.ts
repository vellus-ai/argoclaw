import { describe, it, expect } from "vitest";
import fs from "fs";
import path from "path";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const LOCALES_DIR = path.resolve(
  __dirname,
  "../../../i18n/locales",
);

const SUPPORTED_LOCALES = ["en", "pt", "es", "fr", "it", "de", "vi", "zh"] as const;

interface NestedRecord {
  [key: string]: string | NestedRecord;
}

function loadSetupJson(locale: string): NestedRecord {
  const filePath = path.join(LOCALES_DIR, locale, "setup.json");
  const content = fs.readFileSync(filePath, "utf-8");
  return JSON.parse(content) as NestedRecord;
}

function extractKeys(
  obj: NestedRecord,
  prefix = "",
): string[] {
  const keys: string[] = [];
  for (const [key, value] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key;
    if (typeof value === "string") {
      keys.push(fullKey);
    } else if (typeof value === "object" && value !== null) {
      keys.push(...extractKeys(value as NestedRecord, fullKey));
    }
  }
  return keys.sort();
}

function getValueAtPath(obj: NestedRecord, dotPath: string): string | undefined {
  const parts = dotPath.split(".");
  let current: NestedRecord | string | undefined = obj;
  for (const part of parts) {
    if (typeof current !== "object" || current === null) return undefined;
    current = (current as NestedRecord)[part];
  }
  return typeof current === "string" ? current : undefined;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("i18n consistency — setup.json onboarding section", () => {
  const localeData = new Map<string, NestedRecord>();
  const onboardingKeys = new Map<string, string[]>();

  // Load all locales
  for (const locale of SUPPORTED_LOCALES) {
    const data = loadSetupJson(locale);
    localeData.set(locale, data);
    const onboarding = data.onboarding;
    if (typeof onboarding === "object" && onboarding !== null) {
      onboardingKeys.set(
        locale,
        extractKeys(onboarding as NestedRecord, "onboarding"),
      );
    } else {
      onboardingKeys.set(locale, []);
    }
  }

  it("should have an onboarding section in all locales", () => {
    for (const locale of SUPPORTED_LOCALES) {
      const data = localeData.get(locale);
      expect(
        data?.onboarding,
        `Locale "${locale}" is missing the "onboarding" section`,
      ).toBeDefined();
    }
  });

  // For each locale, verify that all keys present in "en" also exist in that locale
  for (const locale of SUPPORTED_LOCALES) {
    if (locale === "en") continue;

    it(`should have all English onboarding keys in locale "${locale}"`, () => {
      const enKeys = onboardingKeys.get("en") ?? [];
      const localeKeySet = new Set(onboardingKeys.get(locale) ?? []);

      const missingKeys: string[] = [];
      for (const key of enKeys) {
        if (!localeKeySet.has(key)) {
          missingKeys.push(key);
        }
      }

      expect(
        missingKeys,
        `Locale "${locale}" is missing keys: ${missingKeys.join(", ")}`,
      ).toHaveLength(0);
    });
  }

  // Verify that each locale doesn't have extra keys not in English
  for (const locale of SUPPORTED_LOCALES) {
    if (locale === "en") continue;

    it(`should not have extra onboarding keys in locale "${locale}" absent from "en"`, () => {
      const enKeySet = new Set(onboardingKeys.get("en") ?? []);
      const extraKeys: string[] = [];

      for (const key of (onboardingKeys.get(locale) ?? [])) {
        if (!enKeySet.has(key)) {
          extraKeys.push(key);
        }
      }

      expect(
        extraKeys,
        `Locale "${locale}" has extra keys: ${extraKeys.join(", ")}`,
      ).toHaveLength(0);
    });
  }

  // Verify no key has an empty value
  for (const locale of SUPPORTED_LOCALES) {
    it(`should have no empty onboarding values in locale "${locale}"`, () => {
      const emptyKeys: string[] = [];
      const data = localeData.get(locale);
      if (!data) return;

      for (const key of (onboardingKeys.get(locale) ?? [])) {
        const value = getValueAtPath(data, key);
        if (value !== undefined && value.trim() === "") {
          emptyKeys.push(key);
        }
      }

      expect(
        emptyKeys,
        `Locale "${locale}" has empty values: ${emptyKeys.join(", ")}`,
      ).toHaveLength(0);
    });
  }

  // Verify total key count matches across all locales
  it("should have the same number of onboarding keys in all locales", () => {
    const enCount = (onboardingKeys.get("en") ?? []).length;
    for (const locale of SUPPORTED_LOCALES) {
      const count = (onboardingKeys.get(locale) ?? []).length;
      expect(
        count,
        `Locale "${locale}" has ${count} keys, expected ${enCount}`,
      ).toBe(enCount);
    }
  });

  // Verify interpolation variables are consistent across locales
  it("should have matching interpolation variables across all locales", () => {
    const varPattern = /\{\{(\w+)\}\}/g;
    const enKeys = onboardingKeys.get("en") ?? [];
    const enData = localeData.get("en");
    if (!enData) return;

    for (const key of enKeys) {
      const enValue = getValueAtPath(enData, key);
      if (!enValue) continue;

      const enVars = [...enValue.matchAll(varPattern)].map((m) => m[1]).sort();
      if (enVars.length === 0) continue;

      for (const locale of SUPPORTED_LOCALES) {
        if (locale === "en") continue;

        const data = localeData.get(locale);
        if (!data) continue;

        const localeValue = getValueAtPath(data, key);
        if (!localeValue) continue;

        const localeVars = [...localeValue.matchAll(varPattern)]
          .map((m) => m[1])
          .sort();

        expect(
          localeVars,
          `Key "${key}" in "${locale}" has variables [${localeVars.join(",")}] but "en" has [${enVars.join(",")}]`,
        ).toEqual(enVars);
      }
    }
  });
});
