import { useState, useMemo } from "react";

type SortDirection = "asc" | "desc";

interface SortState {
  key: string;
  direction: SortDirection;
}

export function useSortable<T>(data: T[], defaultKey = "", defaultDir: SortDirection = "asc") {
  const [sort, setSort] = useState<SortState>({ key: defaultKey, direction: defaultDir });

  const sorted = useMemo(() => {
    if (!sort.key) return data;
    return [...data].sort((a, b) => {
      const aVal = getNestedValue(a, sort.key);
      const bVal = getNestedValue(b, sort.key);

      if (aVal == null && bVal == null) return 0;
      if (aVal == null) return 1;
      if (bVal == null) return -1;

      let cmp: number;
      if (typeof aVal === "number" && typeof bVal === "number") {
        cmp = aVal - bVal;
      } else {
        cmp = String(aVal).localeCompare(String(bVal), undefined, { sensitivity: "base" });
      }

      return sort.direction === "desc" ? -cmp : cmp;
    });
  }, [data, sort]);

  const toggleSort = (key: string) => {
    setSort((prev) => ({
      key,
      direction: prev.key === key && prev.direction === "asc" ? "desc" : "asc",
    }));
  };

  return { sorted, sort, toggleSort };
}

function getNestedValue(obj: unknown, path: string): unknown {
  return path.split(".").reduce((acc: unknown, part) => {
    if (acc && typeof acc === "object") return (acc as Record<string, unknown>)[part];
    return undefined;
  }, obj);
}
