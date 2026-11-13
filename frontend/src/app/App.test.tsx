import { fireEvent, render, screen } from "@testing-library/react";
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

const columns = [
  { id: "c1", name: "id", type: "integer", nullability: "not_null" },
  { id: "c2", name: "name", type: "text", nullability: "not_null" },
  { id: "c3", name: "salary", type: "integer", nullability: "nullable" },
  { id: "c4", name: "dept_id", type: "integer", nullability: "nullable" },
  { id: "c5", name: "active", type: "boolean", nullability: "not_null" },
];

const datasetsBody = {
  datasets: [
    {
      id: "samples",
      name: "samples",
      createdAt: "2026-11-13T10:00:00Z",
      source: { filename: "built-in", bytes: 0 },
      tableCount: 2,
      rowCount: 12,
      tables: [
        { id: "departments", name: "departments", rowCount: 4, columnCount: 4 },
        { id: "employees", name: "employees", rowCount: 8, columnCount: 5 },
      ],
    },
  ],
};

const tableBody = {
  id: "employees",
  name: "employees",
  rowCount: 8,
  columnCount: 5,
  columns,
};

const rowsBody = {
  tableId: "employees",
  columns,
  rows: [["1", "Ada Lovelace", "145000", "10", true]],
  offset: 0,
  limit: 50,
  totalRows: 8,
  hasMore: true,
};

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function requestPath(input: RequestInfo | URL): string {
  const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
  return new URL(url).pathname;
}

function mockApi() {
  return vi.fn(async (input: RequestInfo | URL) => {
    const path = requestPath(input);
    switch (path) {
      case "/api/v1/health":
        return jsonResponse(healthBody);
      case "/api/v1/datasets":
        return jsonResponse(datasetsBody);
      case "/api/v1/datasets/samples/tables/employees":
        return jsonResponse(tableBody);
      case "/api/v1/datasets/samples/tables/employees/rows":
        return jsonResponse(rowsBody);
      default:
        return jsonResponse(
          { error: { code: "not_found", message: `No mock for ${path}` } },
          404,
        );
    }
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
    vi.stubGlobal("fetch", mockApi());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders the dataset explorer with the sample tables", async () => {
    renderApp();

    expect(await screen.findByRole("heading", { name: /select a table/i })).toBeInTheDocument();
    expect(screen.getByText("samples")).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: /employees/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /departments/i })).toBeInTheDocument();
  });

  it("reports the API as connected once health resolves", async () => {
    renderApp();

    expect(await screen.findAllByText("API connected")).not.toHaveLength(0);
    expect(await screen.findAllByText("v0.1.0-dev")).not.toHaveLength(0);
  });

  it("shows the schema and a row preview for a selected table", async () => {
    renderApp();

    fireEvent.click(await screen.findByRole("button", { name: /employees/i }));

    expect(await screen.findByText("Ada Lovelace")).toBeInTheDocument();
    expect(await screen.findByText("145000")).toBeInTheDocument();
    expect(screen.getAllByText("salary").length).toBeGreaterThan(0);
    expect(screen.getByRole("heading", { name: "employees" })).toBeInTheDocument();
  });

  it("explains how to start the API when it is unreachable", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("Failed to fetch");
      }),
    );

    renderApp();

    expect(
      await screen.findByRole("heading", { name: /api connection/i }, { timeout: 5_000 }),
    ).toBeInTheDocument();
    expect(await screen.findByText(/npm run dev:api/, {}, { timeout: 5_000 })).toBeInTheDocument();
  });
});
