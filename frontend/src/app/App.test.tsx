import { fireEvent, render, screen, within } from "@testing-library/react";
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

const queryColumns = [
  { id: "c1", name: "name", type: "text" },
  { id: "c2", name: "salary", type: "integer" },
];

const queryBody = {
  queryRunId: "run-1",
  status: "completed",
  datasetId: "samples",
  columns: queryColumns,
  rows: [["Ada Lovelace", "145000"]],
  rowCount: 1,
  resultLimit: 1000,
  durationMs: 1.5,
  execution: {
    rootId: "n1",
    nodes: [
      {
        id: "n1",
        kind: "project",
        label: "name, salary",
        children: ["n2"],
        columns: queryColumns,
        rowsIn: 8,
        rowsOut: 1,
        durationMs: 0.5,
        status: "completed",
        input: {
          columns,
          rows: [["1", "Ada Lovelace", "145000", "10", true]],
          truncated: true,
        },
        output: { columns: queryColumns, rows: [["Ada Lovelace", "145000"]], truncated: false },
      },
      {
        id: "n2",
        kind: "scan",
        label: "employees",
        parentId: "n1",
        children: [],
        columns,
        rowsIn: 0,
        rowsOut: 8,
        durationMs: 0.3,
        status: "completed",
        input: { columns: [], rows: [], truncated: false },
        output: {
          columns,
          rows: [["1", "Ada Lovelace", "145000", "10", true]],
          truncated: true,
        },
      },
    ],
    edges: [{ from: "n1", to: "n2" }],
    durationMs: 0.8,
    status: "completed",
  },
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
  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
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
      case "/api/v1/queries": {
        const body = JSON.parse(String(init?.body ?? "{}")) as { sql?: string };
        if ((body.sql ?? "").includes('WHERE "id" > 0')) {
          return jsonResponse(
            {
              error: {
                code: "type_error",
                message: "WHERE requires a boolean predicate, found integer.",
                location: { line: 1, column: 1 },
              },
            },
            400,
          );
        }
        return jsonResponse(queryBody);
      }
      default:
        return jsonResponse({ error: { code: "not_found", message: `No mock for ${path}` } }, 404);
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

  it("runs a query from the query tab and shows the backend result", async () => {
    renderApp();

    fireEvent.click(await screen.findByRole("button", { name: /employees/i }));
    fireEvent.click(screen.getByRole("tab", { name: "Query" }));

    const results = screen.getByRole("region", { name: /query results/i });
    expect(within(results).getByText("No results yet")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /^run$/i }));

    expect(await within(results).findByText("Ada Lovelace")).toBeInTheDocument();
    expect(within(results).getByText("145000")).toBeInTheDocument();
    expect(within(results).getByText("1 row")).toBeInTheDocument();
    expect(within(results).getByText("limit 1000")).toBeInTheDocument();
  });

  it("preserves the last successful result and marks it stale after a failure", async () => {
    renderApp();

    fireEvent.click(await screen.findByRole("button", { name: /employees/i }));
    fireEvent.click(screen.getByRole("tab", { name: "Query" }));

    const results = screen.getByRole("region", { name: /query results/i });
    fireEvent.click(screen.getByRole("button", { name: /^run$/i }));
    expect(await within(results).findByText("Ada Lovelace")).toBeInTheDocument();

    const exampleSelect = screen.getByLabelText("Example queries");
    const failingExample = within(exampleSelect).getByRole("option", {
      name: "Filter on id",
    }) as HTMLOptionElement;
    fireEvent.change(exampleSelect, { target: { value: failingExample.value } });
    fireEvent.click(screen.getByRole("button", { name: /^run$/i }));

    expect(await within(results).findByText("Query failed")).toBeInTheDocument();
    expect(within(results).getByText("type_error")).toBeInTheDocument();
    expect(within(results).getByText(/showing the last successful result/i)).toBeInTheDocument();
    expect(within(results).getByText("Ada Lovelace")).toBeInTheDocument();
  });

  it("shows the execution plan and inspects an operator after a run", async () => {
    renderApp();

    fireEvent.click(await screen.findByRole("button", { name: /employees/i }));
    fireEvent.click(screen.getByRole("tab", { name: "Query" }));
    fireEvent.click(screen.getByRole("button", { name: /^run$/i }));
    expect(await screen.findByText("Ada Lovelace")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Execution plan" }));

    const plan = screen.getByRole("region", { name: "Execution plan" });
    expect(within(plan).getByText("2 operators")).toBeInTheDocument();
    expect(within(plan).getByText("Project")).toBeInTheDocument();
    expect(within(plan).getByText("Scan")).toBeInTheDocument();

    fireEvent.click(within(plan).getByText("Scan"));

    expect(within(plan).getByText("OPERATOR · n2")).toBeInTheDocument();
    expect(within(plan).getAllByText("employees").length).toBeGreaterThan(0);
  });
});
