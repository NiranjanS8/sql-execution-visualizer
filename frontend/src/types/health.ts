export type HealthResponse = {
  status: string;
  service: string;
  version: string;
  time: string;
  uptimeMs: number;
};
