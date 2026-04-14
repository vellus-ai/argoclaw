import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { axe } from "vitest-axe";
import { ErrorState } from "../error-state";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        "retry": "Retry",
      };
      return translations[key] ?? key;
    },
    i18n: { changeLanguage: vi.fn() },
  }),
  initReactI18next: { type: "3rdParty", init: vi.fn() },
}));

describe("ErrorState", () => {
  it("renders message with role='alert'", () => {
    render(<ErrorState message="Something went wrong" />);
    const alert = screen.getByRole("alert");
    expect(alert).toBeInTheDocument();
    expect(alert).toHaveTextContent("Something went wrong");
  });

  it("has aria-live='assertive' on the alert container", () => {
    render(<ErrorState message="Error occurred" />);
    const alert = screen.getByRole("alert");
    expect(alert).toHaveAttribute("aria-live", "assertive");
  });

  it("renders retry button when onRetry is provided", () => {
    render(<ErrorState message="Error" onRetry={() => {}} />);
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("does not render retry button when onRetry is omitted", () => {
    render(<ErrorState message="Error" />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("calls onRetry when retry button is clicked", async () => {
    const onRetry = vi.fn();
    render(<ErrorState message="Error" onRetry={onRetry} />);
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("propagates className", () => {
    const { container } = render(
      <ErrorState message="Error" className="custom-class" />
    );
    expect(container.firstElementChild?.className).toContain("custom-class");
  });

  it("passes accessibility check (vitest-axe)", async () => {
    const { container } = render(
      <ErrorState message="Something went wrong" onRetry={() => {}} />
    );
    const results = await axe(container);
    expect(results).toHaveNoViolations();
  });
});
