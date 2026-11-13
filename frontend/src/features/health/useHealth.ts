import { useQuery } from "@tanstack/react-query";

import { fetchHealth } from "../../lib/api/health";

export function useHealth() {
  return useQuery({
    queryKey: ["health"],
    queryFn: ({ signal }) => fetchHealth(signal),
    refetchInterval: 15_000,
  });
}
