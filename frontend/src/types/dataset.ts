export type ColumnType = "boolean" | "integer" | "decimal" | "text";

export type Nullability = "unknown" | "nullable" | "not_null";

export type DatasetSource = {
  filename: string;
  bytes: number;
};

export type TableSummary = {
  id: string;
  name: string;
  rowCount: number;
  columnCount: number;
};

export type DatasetSummary = {
  id: string;
  name: string;
  createdAt: string;
  source: DatasetSource;
  tableCount: number;
  rowCount: number;
};

export type DatasetDetail = DatasetSummary & {
  tables: TableSummary[];
};

export type Column = {
  id: string;
  name: string;
  type: ColumnType;
  nullability: Nullability;
};

export type TableDetail = TableSummary & {
  columns: Column[];
};

// Numeric cells arrive as strings so int64 values survive JavaScript unchanged.
export type Cell = string | boolean | null;

export type RowsPage = {
  tableId: string;
  columns: Column[];
  rows: Cell[][];
  offset: number;
  limit: number;
  totalRows: number;
  hasMore: boolean;
};

export type PreviewColumn = {
  name: string;
  type: ColumnType;
  nullability: Nullability;
  nullCount: number;
};

export type ImportPreview = {
  tableName: string;
  columns: PreviewColumn[];
  rows: Cell[][];
  rowCount: number;
  byteCount: number;
  rowLimit: number;
  warnings: string[];
  nullValues: string[];
};
