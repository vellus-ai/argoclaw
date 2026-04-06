import { describe, it, expect } from "vitest";
import { getJwtExp } from "./use-jwt-refresh";

// Helper: create a valid JWT with given payload
function makeJwt(payload: Record<string, unknown>): string {
  const header = btoa(JSON.stringify({ alg: "HS256", typ: "JWT" }));
  const body = btoa(JSON.stringify(payload))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
  return `${header}.${body}.fakesignature`;
}

describe("getJwtExp", () => {
  it("should return exp from a valid JWT", () => {
    const exp = Math.floor(Date.now() / 1000) + 900;
    const token = makeJwt({ uid: "user-1", exp });
    expect(getJwtExp(token)).toBe(exp);
  });

  it("should return null for a gateway token (no dots)", () => {
    expect(getJwtExp("abc123def456")).toBeNull();
  });

  it("should return null for a token with only 2 parts", () => {
    expect(getJwtExp("header.payload")).toBeNull();
  });

  it("should return null for an empty string", () => {
    expect(getJwtExp("")).toBeNull();
  });

  it("should return null when payload has no exp field", () => {
    const token = makeJwt({ uid: "user-1", email: "test@test.com" });
    expect(getJwtExp(token)).toBeNull();
  });

  it("should return null when exp is a string instead of number", () => {
    const token = makeJwt({ uid: "user-1", exp: "not-a-number" });
    expect(getJwtExp(token)).toBeNull();
  });

  it("should return null for invalid base64 in payload", () => {
    expect(getJwtExp("header.!!!invalid!!!.signature")).toBeNull();
  });

  it("should return null when payload is not valid JSON", () => {
    const header = btoa(JSON.stringify({ alg: "HS256" }));
    const body = btoa("not json at all");
    expect(getJwtExp(`${header}.${body}.sig`)).toBeNull();
  });

  it("should handle URL-safe base64 characters (- and _)", () => {
    const exp = 1700000000;
    // Create payload with characters that produce + and / in standard base64
    const payload = { uid: "user-with-special-chars-???", exp };
    const token = makeJwt(payload);
    expect(getJwtExp(token)).toBe(exp);
  });

  it("should handle base64 payloads that need padding", () => {
    // Create a payload whose base64 length is not a multiple of 4
    const exp = 1700000001;
    const payload = { a: "x", exp }; // short payload → likely needs padding
    const token = makeJwt(payload);
    expect(getJwtExp(token)).toBe(exp);
  });

  it("should return exp=0 as a valid number", () => {
    const token = makeJwt({ exp: 0 });
    expect(getJwtExp(token)).toBe(0);
  });

  it("should return negative exp as valid (expired long ago)", () => {
    const token = makeJwt({ exp: -100 });
    expect(getJwtExp(token)).toBe(-100);
  });
});

// Note: The useJwtRefresh hook integrates getJwtExp (tested above) with
// refreshTokenSingleton (tested in token-refresh.test.ts). Full hook tests
// would require @testing-library/react + renderHook which is not set up.
// The critical paths (JWT decode, singleton dedup, store update, logout on
// failure) are covered by the unit tests in this file and token-refresh.test.ts.
