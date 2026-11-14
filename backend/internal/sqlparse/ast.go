// Package sqlparse turns SQL text into QueryLens-owned structures.
//
// The grammar is a deliberately small subset: one SELECT over one table, with a
// projection, an optional WHERE predicate, an optional ORDER BY, and an optional
// LIMIT. Anything else is rejected rather than approximated, and every rejection
// carries a code and, where possible, a source position.
//
// Nothing here knows about HTTP, execution, or the frontend. The validator in
// internal/validate consumes these structures and fills in column bindings.
package sqlparse

import "github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"

// Position is a 1-based source location.
type Position struct {
	Line   int
	Column int
}

// Statement is a parsed SQL statement. Only *Select is produced today.
type Statement interface {
	statementNode()
}

// Select is a single-table SELECT statement.
type Select struct {
	Items   []SelectItem
	From    TableRef
	Where   Expression // nil when the statement has no WHERE clause
	OrderBy []OrderItem
	Limit   *int // nil when the statement has no LIMIT clause
	Pos     Position
}

func (*Select) statementNode() {}

// TableRef names the table in FROM and its optional alias.
type TableRef struct {
	Name  string
	Alias string
	Pos   Position
}

// SelectItem is one entry in the projection.
type SelectItem struct {
	Expr  Expression
	Alias string // empty when the item has no alias
	Pos   Position
}

// OrderItem is one ORDER BY term.
type OrderItem struct {
	Expr Expression
	Desc bool
	Pos  Position
}

// Expression is any evaluable node. Star is only valid as a SelectItem.Expr.
type Expression interface {
	exprNode()
	Position() Position
}

// Star is an unqualified (*) or qualified (t.*) wildcard in the projection.
type Star struct {
	Table string // empty for *
	Pos   Position
}

func (*Star) exprNode()            {}
func (s *Star) Position() Position { return s.Pos }

// ColumnRef is a column reference, optionally qualified by a table name or
// alias. Resolved is filled in by internal/validate and is nil before then.
type ColumnRef struct {
	Table    string
	Name     string
	Pos      Position
	Resolved *ResolvedColumn
}

func (*ColumnRef) exprNode()            {}
func (c *ColumnRef) Position() Position { return c.Pos }

// ResolvedColumn is the catalog column a ColumnRef binds to. Index is the
// column's position within Source.Schema.
type ResolvedColumn struct {
	Source *catalog.Table
	Column catalog.Column
	Index  int
}

// Literal is a typed constant. NULL is catalog.Null().
type Literal struct {
	Value catalog.Value
	Pos   Position
}

func (*Literal) exprNode()            {}
func (l *Literal) Position() Position { return l.Pos }

// UnaryOp is a prefix operator.
type UnaryOp int

const (
	OpNot UnaryOp = iota
	OpNegate
)

// Unary is a prefix NOT or numeric negation.
type Unary struct {
	Op      UnaryOp
	Operand Expression
	Pos     Position
}

func (*Unary) exprNode()            {}
func (u *Unary) Position() Position { return u.Pos }

// BinaryOp is an infix operator.
type BinaryOp int

const (
	OpOr BinaryOp = iota
	OpAnd
	OpEqual
	OpNotEqual
	OpLess
	OpLessEqual
	OpGreater
	OpGreaterEqual
	OpAdd
	OpSubtract
	OpMultiply
	OpDivide
)

// Binary is an infix expression. Pos is the start of the left operand.
type Binary struct {
	Op    BinaryOp
	Left  Expression
	Right Expression
	Pos   Position
}

func (*Binary) exprNode()            {}
func (b *Binary) Position() Position { return b.Pos }

// IsNull is a postfix IS NULL or IS NOT NULL test.
type IsNull struct {
	Operand Expression
	Negated bool
	Pos     Position
}

func (*IsNull) exprNode()            {}
func (n *IsNull) Position() Position { return n.Pos }
