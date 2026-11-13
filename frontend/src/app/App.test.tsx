import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { App } from "./App";
import { AppProviders } from "./providers/AppProviders";

const healthBody = {
  status: "ok",
  service: "querylens",
  version: "0.1.0-dev",
  time: "2026-11-13T10:12:31Z",
  uptimeMs: 4_200,
};

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function renderApp() {
  return render(
    <AppProviders>
      <App />
    </AppProviders>,
  );
}

describe("App", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse(healthBody)),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders the workspace shell", () => {
    renderApp();

    expect(screen.getByRole("heading", { name: /execution workbench/i })).toBeInTheDocument();
    expect(screen.getByText("QueryLens")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /api connection/i })).toBeInTheDocument();
  });

  it("reports the API as connected once health resolves", async () => {
    renderApp();

    expect(await screen.findAllByText("API connected")).not.toHaveLength(0);
    expect(await screen.findAllByText("v0.1.0-dev")).not.toHaveLength(0);
    expect(await screen.findByText("4s")).toBeInTheDocument();
  });

  it("explains how to start the API when it is unreachable", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("Failed to fetch");
      }),
    );

    renderApp();

    expect(await screen.findByText("Failed to fetch", {}, { timeout: 5_000 })).toBeInTheDocument();
    expect(screen.getByText(/npm run dev:api/)).toBeInTheDocument();
  });
});
