import { describe, it, expect, vi, beforeEach } from "vitest";
import { login, register, refresh, AuthApiError } from "./auth-client";

const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

function jsonResponse(status: number, body: unknown) {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? "OK" : "Error",
    json: () => Promise.resolve(body),
  };
}

beforeEach(() => {
  mockFetch.mockReset();
});

describe("login", () => {
  it("should return auth response on success", async () => {
    const authRes = {
      access_token: "at-123",
      refresh_token: "rt-456",
      expires_in: 900,
      user: { id: "u1", email: "a@b.com", display_name: "Test", role: "owner", status: "active" },
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(200, authRes));

    const result = await login("a@b.com", "password123");

    expect(result).toEqual(authRes);
    expect(mockFetch).toHaveBeenCalledWith("/v1/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email: "a@b.com", password: "password123" }),
    });
  });

  it("should throw AuthApiError on 401", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse(401, { error: "invalid credentials" }));

    try {
      await login("a@b.com", "wrong");
      expect.unreachable("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(AuthApiError);
      expect((err as AuthApiError).status).toBe(401);
    }
  });

  it("should throw AuthApiError on 429", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse(429, { error: "rate limited" }));

    try {
      await login("a@b.com", "test");
    } catch (err) {
      expect(err).toBeInstanceOf(AuthApiError);
      expect((err as AuthApiError).status).toBe(429);
    }
  });
});

describe("register", () => {
  it("should send display_name when provided", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse(201, {
      access_token: "at", refresh_token: "rt", expires_in: 900,
      user: { id: "u1", email: "a@b.com", display_name: "John", role: "owner", status: "active" },
    }));

    await register("a@b.com", "StrongPass123!", "John");

    const body = JSON.parse(mockFetch.mock.calls[0]![1].body as string);
    expect(body.display_name).toBe("John");
  });

  it("should omit display_name when not provided", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse(201, {
      access_token: "at", refresh_token: "rt", expires_in: 900,
      user: { id: "u1", email: "a@b.com", display_name: "", role: "owner", status: "active" },
    }));

    await register("a@b.com", "StrongPass123!");

    const body = JSON.parse(mockFetch.mock.calls[0]![1].body as string);
    expect(body).not.toHaveProperty("display_name");
  });

  it("should throw AuthApiError on 409 (email taken)", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse(409, { error: "email already registered" }));

    try {
      await register("taken@b.com", "StrongPass123!");
    } catch (err) {
      expect(err).toBeInstanceOf(AuthApiError);
      expect((err as AuthApiError).status).toBe(409);
    }
  });
});

describe("refresh", () => {
  it("should send refresh_token in body", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse(200, {
      access_token: "new-at", refresh_token: "new-rt", expires_in: 900,
      user: { id: "u1", email: "a@b.com", display_name: "", role: "owner", status: "active" },
    }));

    const result = await refresh("old-rt");

    expect(result.access_token).toBe("new-at");
    const body = JSON.parse(mockFetch.mock.calls[0]![1].body as string);
    expect(body.refresh_token).toBe("old-rt");
  });
});
