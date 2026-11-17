import { apiRequest } from "./client";
import type { QueryResult } from "../../types/query";

export function runQuery(
  datasetId: string,
  sql: string,
  signal?: AbortSignal,
): Promise<QueryResult> {
  return apiRequest<QueryResult>("/api/v1/queries", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ datasetId, sql }),
    signal,
  });
}
