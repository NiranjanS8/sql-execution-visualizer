import { cn } from "../../lib/cn";
import type { Cell, ColumnType } from "../../types/dataset";

type GridColumn = {
  name: string;
  type: ColumnType;
};

type DataGridProps = {
  columns: GridColumn[];
  rows: Cell[][];
  startIndex?: number;
  emptyMessage?: string;
};

function isNumeric(type: ColumnType | undefined): boolean {
  return type === "integer" || type === "decimal";
}

function CellValue({ cell }: { cell: Cell }) {
  if (cell === null) {
    return <span className="text-[11px] italic text-ink-faint">NULL</span>;
  }
  if (typeof cell === "boolean") {
    return <span className="text-ink-muted">{cell ? "true" : "false"}</span>;
  }
  if (cell === "") {
    return <span className="text-ink-faint">""</span>;
  }
  return (
    <span className="block max-w-[22rem] truncate" title={cell}>
      {cell}
    </span>
  );
}

export function DataGrid({
  columns,
  rows,
  startIndex = 0,
  emptyMessage = "No rows.",
}: DataGridProps) {
  if (rows.length === 0) {
    return <p className="px-4 py-8 text-center text-xs text-ink-faint">{emptyMessage}</p>;
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-left text-xs">
        <thead>
          <tr className="border-b border-line">
            <th
              scope="col"
              className="w-12 px-3 py-2 text-right font-mono text-[10px] font-normal text-ink-faint"
            >
              #
            </th>
            {columns.map((column, index) => (
              <th key={`${column.name}-${index}`} scope="col" className="px-3 py-2 align-bottom">
                <span className="block font-mono text-[11px] font-medium text-ink">
                  {column.name}
                </span>
                <span className="mt-0.5 block text-[10px] font-normal tracking-[0.06em] text-ink-faint uppercase">
                  {column.type}
                </span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, rowIndex) => (
            <tr
              key={startIndex + rowIndex}
              className="border-b border-line/60 last:border-0 hover:bg-hover/50"
            >
              <td className="px-3 py-1.5 text-right font-mono text-[10px] text-ink-faint">
                {startIndex + rowIndex + 1}
              </td>
              {row.map((cell, cellIndex) => (
                <td
                  key={cellIndex}
                  className={cn(
                    "px-3 py-1.5",
                    isNumeric(columns[cellIndex]?.type)
                      ? "text-right font-mono text-ink"
                      : "text-ink-muted",
                  )}
                >
                  <CellValue cell={cell} />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
