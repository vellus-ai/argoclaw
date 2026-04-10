import { describe, it, expect, vi, beforeEach, type Mock } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import "@/i18n";
import { LoginPage } from "./login-page";

// Helper: wrap LoginPage with MemoryRouter for useNavigate/useLocation
function renderLoginPage() {
  return render(
    <MemoryRouter initialEntries={["/login"]}>
      <LoginPage />
    </MemoryRouter>,
  );
}

// Helper: create a deferred promise for controlling fetch timing
function createDeferredFetch() {
  let resolve!: (res: Response) => void;
  let reject!: (err: Error) => void;
  const promise = new Promise<Response>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe("LoginPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe("email auth available (200/405 response)", () => {
    it("should render EmailForm without LoginTabs when endpoint returns 200", async () => {
      vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
        new Response(null, { status: 200 }),
      );

      renderLoginPage();

      // Wait for useEffect to resolve
      await waitFor(() => {
        expect(globalThis.fetch).toHaveBeenCalledWith("/v1/auth/login", {
          method: "HEAD",
        });
      });

      // EmailForm renders email input
      expect(screen.getByLabelText("Email")).toBeInTheDocument();
      // LoginTabs should NOT be visible (tabs: Email, Token, Pairing)
      expect(screen.queryByRole("button", { name: "Token" })).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Pairing" })).not.toBeInTheDocument();
    });

    it("should render EmailForm without LoginTabs when endpoint returns 405", async () => {
      vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
        new Response(null, { status: 405 }),
      );

      renderLoginPage();

      await waitFor(() => {
        expect(globalThis.fetch).toHaveBeenCalled();
      });

      // EmailForm is rendered (email label exists)
      expect(screen.getByLabelText("Email")).toBeInTheDocument();
      // No tabs shown
      expect(screen.queryByRole("button", { name: "Token" })).not.toBeInTheDocument();
    });
  });

  describe("email auth unavailable (404 response)", () => {
    it("should show LoginTabs and default to token mode", async () => {
      vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
        new Response(null, { status: 404 }),
      );

      renderLoginPage();

      // Wait for tabs to appear (emailAuthAvailable becomes false -> showTabs=true)
      await waitFor(() => {
        expect(screen.getByRole("button", { name: "Token" })).toBeInTheDocument();
      });

      // All three tabs visible
      expect(screen.getByRole("button", { name: "Email" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Token" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Pairing" })).toBeInTheDocument();

      // TokenForm is rendered (mode switched to "token" automatically)
      // TokenForm has "User ID" and "Gateway Token" labels
      expect(screen.getByLabelText("User ID")).toBeInTheDocument();
      expect(screen.getByLabelText("Gateway Token")).toBeInTheDocument();
    });
  });

  describe("fetch network error", () => {
    it("should fall back to showing LoginTabs when fetch throws", async () => {
      vi.spyOn(globalThis, "fetch").mockRejectedValueOnce(
        new TypeError("Failed to fetch"),
      );

      renderLoginPage();

      // Tabs should appear after catch sets emailAuthAvailable=false
      await waitFor(() => {
        expect(screen.getByRole("button", { name: "Token" })).toBeInTheDocument();
      });

      // TokenForm rendered (fallback to token mode)
      expect(screen.getByLabelText("User ID")).toBeInTheDocument();
    });
  });

  describe("loading state (before fetch resolves)", () => {
    it("should show EmailForm while detection is pending (no flash)", async () => {
      const deferred = createDeferredFetch();
      vi.spyOn(globalThis, "fetch").mockReturnValueOnce(deferred.promise);

      renderLoginPage();

      // While emailAuthAvailable is null (pending), EmailForm is shown
      // (mode defaults to "email", showTabs is false because null !== false)
      expect(screen.getByLabelText("Email")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Token" })).not.toBeInTheDocument();

      // Resolve to confirm it transitions correctly
      deferred.resolve(new Response(null, { status: 200 }));

      await waitFor(() => {
        expect(globalThis.fetch).toHaveBeenCalled();
      });

      // Still showing EmailForm, no tabs
      expect(screen.getByLabelText("Email")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Token" })).not.toBeInTheDocument();
    });
  });
});
