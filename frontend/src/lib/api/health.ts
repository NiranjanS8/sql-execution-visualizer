import type { HealthResponse } from "../../types/health";
import { apiRequest } from "./client";

export function fetchHealth(signal?: AbortSignal): Promise<HealthResponse> {
  return apiRequest<HealthResponse>("/api/v1/health", { signal });
}
