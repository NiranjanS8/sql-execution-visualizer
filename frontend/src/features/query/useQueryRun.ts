import { useCallback, useEffect, useRef, useState } from "react";

import { ApiError } from "../../lib/api/client";
import { runQuery } from "../../lib/api/queries";
import type { QueryResult } from "../../types/query";

export type QueryRunStatus = "idle" | "running" | "completed" | "failed" | "cancelled";

export type QueryRunState = {
  result: QueryResult | null;
  error: ApiError | null;
  status: QueryRunStatus;
  stale: boolean;
  run: (sql: string) => void;
  cancel: () => void;
};

function toApiError(cause: unknown): ApiError {
  if (cause instanceof ApiError) {
    return cause;
  }
  return new ApiError(
    cause instanceof Error ? cause.message : "The query could not be executed.",
    0,
    "unknown_error",
  );
}

export function useQueryRun(datasetId: string): QueryRunState {
  const [result, setResult] = useState<QueryResult | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [status, setStatus] = useState<QueryRunStatus>("idle");
  const [stale, setStale] = useState(false);

  const controller = useRef<AbortController | null>(null);
  const mounted = useRef(true);

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      controller.current?.abort();
    };
  }, []);

  const cancel = useCallback(() => {
    controller.current?.abort();
  }, []);

  const run = useCallback(
    (sql: string) => {
      controller.current?.abort();

      const active = new AbortController();
      controller.current = active;

      const current = () => mounted.current && controller.current === active;

      setStatus("running");
      setError(null);

      void runQuery(datasetId, sql, active.signal)
        .then((next) => {
          if (!current()) {
            return;
          }
          setResult(next);
          setStale(false);
          setStatus("completed");
        })
        .catch((cause: unknown) => {
          if (!current()) {
            return;
          }
          if (active.signal.aborted) {
            setStatus("cancelled");
            setStale(true);
            return;
          }
          setError(toApiError(cause));
          setStatus("failed");
          setStale(true);
        })
        .finally(() => {
          if (controller.current === active) {
            controller.current = null;
          }
        });
    },
    [datasetId],
  );

  return { result, error, status, stale, run, cancel };
}
