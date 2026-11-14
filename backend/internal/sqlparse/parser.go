package sqlparse

import (
	"math"
	"strconv"
	"strings"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
)

// reservedWords cannot be used as bare identifiers. A column that collides with
// one of them must be written as a quoted identifier, for example "order".
var reservedWords = map[string]struct{}{
	"ALL": {}, "ALTER": {}, "AND": {}, "AS": {}, "ASC": {}, "BETWEEN": {},
	"BY": {}, "CASE": {}, "CAST": {}, "CREATE": {}, "CROSS": {}, "DELETE": {},
	"DESC": {}, "DISTINCT": {}, "DROP": {}, "ELSE": {}, "END": {}, "EXCEPT": {},
	"EXISTS": {}, "EXPLAIN": {}, "FALSE": {}, "FETCH": {}, "FOR": {}, "FROM": {},
	"FULL": {}, "GROUP": {}, "HAVING": {}, "ILIKE": {}, "IN": {}, "INNER": {},
	"INSERT": {}, "INTERSECT": {}, "INTO": {}, "IS": {}, "JOIN": {}, "LEFT": {},
	"LIKE": {}, "LIMIT": {}, "NATURAL": {}, "NOT": {}, "NULL": {}, "OFFSET": {},
	"ON": {}, "OR": {}, "ORDER": {}, "OUTER": {}, "OVER": {}, "PARTITION": {},
	"PRAGMA": {}, "RIGHT": {}, "SELECT": {}, "THEN": {}, "TRUE": {}, "UNION": {},
	"UPDATE": {}, "VALUES": {}, "WHEN": {}, "WHERE": {}, "WINDOW": {}, "WITH": {},
}

func isReservedWord(name string) bool {
	_, ok := reservedWords[strings.ToUpper(name)]
	return ok
}

// clauseKeywords begin a clause rather than an expression, so one appearing where
// an expression is required means a trailing comma or a missing item.
var clauseKeywords = map[string]struct{}{
	"AS": {}, "ASC": {}, "BY": {}, "CROSS": {}, "DESC": {}, "EXCEPT": {},
	"FETCH": {}, "FOR": {}, "FROM": {}, "FULL": {}, "GROUP": {}, "HAVING": {},
	"INNER": {}, "INTERSECT": {}, "INTO": {}, "JOIN": {}, "LEFT": {}, "LIMIT": {},
	"NATURAL": {}, "OFFSET": {}, "ON": {}, "ORDER": {}, "RIGHT": {}, "UNION": {},
	"WHERE": {}, "WINDOW": {},
}

func isClauseKeyword(tok token) bool {
	if tok.kind != tokenIdentifier || tok.quoted {
		return false
	}
	_, ok := clauseKeywords[strings.ToUpper(tok.text)]
	return ok
}

type parser struct {
	tokens []token
	index  int
}

// Parse parses one SELECT statement. The returned statement is unvalidated:
// column references are not yet bound to a catalog.
func Parse(src string) (*Select, error) {
	if strings.TrimSpace(src) == "" {
		return nil, errorAt(CodeParseError, 1, 1, "SQL is empty.")
	}

	tokens, err := lex(src)
	if err != nil {
		return nil, err
	}

	p := &parser{tokens: tokens}
	return p.parseStatement()
}

func (p *parser) parseStatement() (*Select, error) {
	tok := p.current()

	if tok.kind == tokenIdentifier {
		switch {
		case p.checkKeyword("SELECT"):
			return p.parseSelect()
		case p.checkKeyword("WITH"):
			return nil, p.unsupported(tok, "Common table expressions (WITH) are not supported.")
		case p.checkKeyword("INSERT"), p.checkKeyword("UPDATE"), p.checkKeyword("DELETE"),
			p.checkKeyword("CREATE"), p.checkKeyword("DROP"), p.checkKeyword("ALTER"),
			p.checkKeyword("EXPLAIN"), p.checkKeyword("PRAGMA"):
			return nil, p.unsupported(tok, "Only SELECT statements are supported.")
		}
	}

	return nil, p.unexpected(tok)
}

func (p *parser) parseSelect() (*Select, error) {
	start := p.advance()
	stmt := &Select{Pos: start.position()}

	if p.checkKeyword("DISTINCT") {
		return nil, p.unsupported(p.current(), "SELECT DISTINCT is not supported.")
	}
	if p.checkKeyword("ALL") {
		return nil, p.unsupported(p.current(), "SELECT ALL is not supported.")
	}

	items, err := p.parseSelectList()
	if err != nil {
		return nil, err
	}
	stmt.Items = items

	if !p.checkKeyword("FROM") {
		if p.current().kind == tokenEOF {
			return nil, p.unsupported(p.current(), "SELECT without FROM is not supported.")
		}
		return nil, errorAt(CodeParseError, p.current().line, p.current().column,
			"Expected FROM, found %s.", p.current().describe())
	}
	p.advance()

	from, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}
	stmt.From = from

	if err := p.rejectUnsupportedClause(); err != nil {
		return nil, err
	}

	if p.checkKeyword("WHERE") {
		p.advance()
		where, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Where = where

		if err := p.rejectUnsupportedClause(); err != nil {
			return nil, err
		}
	}

	if p.checkKeyword("ORDER") {
		p.advance()
		if _, err := p.expectKeyword("BY"); err != nil {
			return nil, err
		}
		orderBy, err := p.parseOrderList()
		if err != nil {
			return nil, err
		}
		stmt.OrderBy = orderBy

		if err := p.rejectUnsupportedClause(); err != nil {
			return nil, err
		}
	}

	if p.checkKeyword("LIMIT") {
		limit, err := p.parseLimit()
		if err != nil {
			return nil, err
		}
		stmt.Limit = limit

		if err := p.rejectUnsupportedClause(); err != nil {
			return nil, err
		}
	}

	return p.finish(stmt)
}

func (p *parser) finish(stmt *Select) (*Select, error) {
	p.match(tokenSemicolon)

	tok := p.current()
	if tok.kind != tokenEOF {
		if tok.kind == tokenSemicolon {
			return nil, errorAt(CodeParseError, tok.line, tok.column, "Only one statement is supported.")
		}
		if err := p.rejectUnsupportedClause(); err != nil {
			return nil, err
		}
		return nil, errorAt(CodeParseError, tok.line, tok.column,
			"Only one statement is supported; unexpected %s.", tok.describe())
	}

	return stmt, nil
}

func (p *parser) parseSelectList() ([]SelectItem, error) {
	var items []SelectItem

	for {
		item, err := p.parseSelectItem()
		if err != nil {
			return nil, err
		}
		items = append(items, item)

		if !p.match(tokenComma) {
			break
		}
		if next := p.current(); next.kind == tokenEOF || isClauseKeyword(next) {
			return nil, errorAt(CodeParseError, next.line, next.column,
				"Expected a select item after \",\".")
		}
	}

	return items, nil
}

func (p *parser) parseSelectItem() (SelectItem, error) {
	tok := p.current()
	item := SelectItem{Pos: tok.position()}

	switch {
	case p.match(tokenStar):
		item.Expr = &Star{Pos: tok.position()}
	case tok.kind == tokenIdentifier && p.peekIs(1, tokenDot) && p.peekIs(2, tokenStar):
		p.advance() // qualifier
		p.advance() // .
		p.advance() // *
		item.Expr = &Star{Table: tok.text, Pos: tok.position()}
	default:
		expr, err := p.parseExpression()
		if err != nil {
			return SelectItem{}, err
		}
		item.Expr = expr
	}

	if p.matchKeyword("AS") {
		alias, err := p.parseAlias()
		if err != nil {
			return SelectItem{}, err
		}
		item.Alias = alias
	} else if tok := p.current(); tok.kind == tokenIdentifier && (tok.quoted || !isReservedWord(tok.text)) {
		item.Alias = p.advance().text
	}

	return item, nil
}

func (p *parser) parseAlias() (string, error) {
	tok := p.current()
	if tok.kind != tokenIdentifier {
		return "", errorAt(CodeParseError, tok.line, tok.column, "Expected an alias after AS, found %s.", tok.describe())
	}
	p.advance()
	return tok.text, nil
}

func (p *parser) parseTableRef() (TableRef, error) {
	tok := p.current()

	if tok.kind == tokenLParen {
		if p.peekIs(1, tokenIdentifier) && strings.EqualFold(p.peek(1).text, "SELECT") {
			return TableRef{}, p.unsupported(tok, "Subqueries are not supported.")
		}
		return TableRef{}, errorAt(CodeParseError, tok.line, tok.column,
			"Expected a table name after FROM, found %s.", tok.describe())
	}
	if tok.kind != tokenIdentifier || (!tok.quoted && isReservedWord(tok.text)) {
		return TableRef{}, errorAt(CodeParseError, tok.line, tok.column,
			"Expected a table name after FROM, found %s.", tok.describe())
	}
	p.advance()

	ref := TableRef{Name: tok.text, Pos: tok.position()}

	if p.matchKeyword("AS") {
		alias, err := p.parseAlias()
		if err != nil {
			return TableRef{}, err
		}
		ref.Alias = alias
	} else if next := p.current(); next.kind == tokenIdentifier && (next.quoted || !isReservedWord(next.text)) {
		ref.Alias = p.advance().text
	}

	return ref, nil
}

func (p *parser) parseOrderList() ([]OrderItem, error) {
	var items []OrderItem

	for {
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		item := OrderItem{Expr: expr, Pos: expr.Position()}
		if p.matchKeyword("DESC") {
			item.Desc = true
		} else {
			p.matchKeyword("ASC")
		}

		if p.checkKeyword("NULLS") {
			return nil, p.unsupported(p.current(), "NULLS FIRST and NULLS LAST are not supported.")
		}

		items = append(items, item)

		if !p.match(tokenComma) {
			break
		}
		if next := p.current(); next.kind == tokenEOF || isClauseKeyword(next) {
			return nil, errorAt(CodeParseError, next.line, next.column,
				"Expected an ORDER BY term after \",\".")
		}
	}

	return items, nil
}

func (p *parser) parseLimit() (*int, error) {
	limitToken := p.current()
	p.advance()

	tok := p.current()
	if tok.kind != tokenInteger {
		return nil, errorAt(CodeParseError, tok.line, tok.column,
			"LIMIT expects a non-negative integer, found %s.", tok.describe())
	}
	p.advance()

	value, err := strconv.ParseInt(tok.text, 10, 64)
	if err != nil || value > maxLimitValue() {
		return nil, errorAt(CodeParseError, tok.line, tok.column,
			"LIMIT %s is out of range.", tok.text)
	}
	if p.current().kind == tokenComma {
		return nil, p.unsupported(limitToken, "LIMIT with an offset (LIMIT n, m) is not supported; use LIMIT n.")
	}

	limit := int(value)
	return &limit, nil
}

func (p *parser) parseExpression() (Expression, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (Expression, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.checkKeyword("OR") {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &Binary{Op: OpOr, Left: left, Right: right, Pos: left.Position()}
	}

	return left, nil
}

func (p *parser) parseAnd() (Expression, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}

	for p.checkKeyword("AND") {
		p.advance()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &Binary{Op: OpAnd, Left: left, Right: right, Pos: left.Position()}
	}

	return left, nil
}

func (p *parser) parseNot() (Expression, error) {
	tok := p.current()
	if p.checkKeyword("NOT") {
		p.advance()
		operand, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &Unary{Op: OpNot, Operand: operand, Pos: tok.position()}, nil
	}
	return p.parseComparison()
}

func (p *parser) parseComparison() (Expression, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}

	if p.checkKeyword("IS") {
		p.advance()
		negated := p.matchKeyword("NOT")
		if !p.checkKeyword("NULL") {
			return nil, p.unsupported(p.current(), "IS is only supported with NULL; use IS NULL or IS NOT NULL.")
		}
		p.advance()
		return &IsNull{Operand: left, Negated: negated, Pos: left.Position()}, nil
	}

	switch {
	case p.checkKeyword("IN"):
		return nil, p.unsupported(p.current(), "IN is not supported; use OR with equality.")
	case p.checkKeyword("BETWEEN"):
		return nil, p.unsupported(p.current(), "BETWEEN is not supported; use two comparisons.")
	case p.checkKeyword("LIKE"), p.checkKeyword("ILIKE"):
		return nil, p.unsupported(p.current(), "LIKE is not supported.")
	}

	op, ok := comparisonOp(p.current().kind)
	if !ok {
		return left, nil
	}
	p.advance()

	right, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}

	if _, chained := comparisonOp(p.current().kind); chained {
		return nil, errorAt(CodeParseError, p.current().line, p.current().column,
			"Comparisons cannot be chained; combine them with AND.")
	}

	return &Binary{Op: op, Left: left, Right: right, Pos: left.Position()}, nil
}

func (p *parser) parseAdditive() (Expression, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return nil, err
	}

	for {
		var op BinaryOp
		switch p.current().kind {
		case tokenPlus:
			op = OpAdd
		case tokenMinus:
			op = OpSubtract
		default:
			return left, nil
		}
		p.advance()

		right, err := p.parseMultiplicative()
		if err != nil {
			return nil, err
		}
		left = &Binary{Op: op, Left: left, Right: right, Pos: left.Position()}
	}
}

func (p *parser) parseMultiplicative() (Expression, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		var op BinaryOp
		switch p.current().kind {
		case tokenStar:
			op = OpMultiply
		case tokenSlash:
			op = OpDivide
		default:
			return left, nil
		}
		p.advance()

		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &Binary{Op: op, Left: left, Right: right, Pos: left.Position()}
	}
}

func (p *parser) parseUnary() (Expression, error) {
	tok := p.current()

	switch {
	case tok.kind == tokenMinus:
		p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &Unary{Op: OpNegate, Operand: operand, Pos: tok.position()}, nil
	case tok.kind == tokenPlus:
		return nil, p.unsupported(tok, "Unary + is not supported.")
	}

	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Expression, error) {
	tok := p.current()

	switch tok.kind {
	case tokenInteger, tokenDecimal:
		p.advance()
		value, err := numericValue(tok)
		if err != nil {
			return nil, err
		}
		return &Literal{Value: value, Pos: tok.position()}, nil

	case tokenString:
		p.advance()
		return &Literal{Value: catalog.Text(tok.text), Pos: tok.position()}, nil

	case tokenLParen:
		p.advance()
		if p.checkKeyword("SELECT") {
			return nil, p.unsupported(p.current(), "Subqueries are not supported.")
		}
		inner, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokenRParen, `")"`); err != nil {
			return nil, err
		}
		return inner, nil

	case tokenIdentifier:
		return p.parseIdentifierExpression(tok)
	}

	return nil, p.unexpected(tok)
}

func (p *parser) parseIdentifierExpression(tok token) (Expression, error) {
	switch strings.ToUpper(tok.text) {
	case "TRUE":
		p.advance()
		return &Literal{Value: catalog.Bool(true), Pos: tok.position()}, nil
	case "FALSE":
		p.advance()
		return &Literal{Value: catalog.Bool(false), Pos: tok.position()}, nil
	case "NULL":
		p.advance()
		return &Literal{Value: catalog.Null(), Pos: tok.position()}, nil
	case "CASE":
		return nil, p.unsupported(tok, "CASE expressions are not supported.")
	case "CAST":
		return nil, p.unsupported(tok, "CAST is not supported.")
	case "EXISTS":
		return nil, p.unsupported(tok, "Subqueries are not supported.")
	}

	if p.peekIs(1, tokenLParen) {
		return nil, p.unsupported(tok, "Function %q is not supported.", tok.text)
	}

	if isReservedWord(tok.text) && !tok.quoted {
		return nil, errorAt(CodeParseError, tok.line, tok.column,
			"%q is a reserved word; quote it as %q to use it as an identifier.", tok.text, tok.text)
	}

	p.advance()
	ref := &ColumnRef{Name: tok.text, Pos: tok.position()}

	if p.match(tokenDot) {
		nameTok := p.current()
		if nameTok.kind != tokenIdentifier {
			return nil, errorAt(CodeParseError, nameTok.line, nameTok.column,
				"Expected a column name after \".\", found %s.", nameTok.describe())
		}
		p.advance()

		ref.Table = ref.Name
		ref.Name = nameTok.text

		if p.current().kind == tokenDot {
			return nil, errorAt(CodeParseError, p.current().line, p.current().column,
				"Column names may have at most one qualifier.")
		}
	}

	return ref, nil
}

func (p *parser) rejectUnsupportedClause() error {
	tok := p.current()
	if tok.kind != tokenIdentifier {
		return nil
	}

	switch strings.ToUpper(tok.text) {
	case "JOIN", "INNER", "LEFT", "RIGHT", "FULL", "CROSS", "NATURAL":
		return p.unsupported(tok, "Joins are not supported; query one table at a time.")
	case "GROUP":
		return p.unsupported(tok, "GROUP BY is not supported.")
	case "HAVING":
		return p.unsupported(tok, "HAVING is not supported.")
	case "UNION", "INTERSECT", "EXCEPT":
		return p.unsupported(tok, "Set operations (UNION, INTERSECT, EXCEPT) are not supported.")
	case "WINDOW":
		return p.unsupported(tok, "Window functions are not supported.")
	case "OFFSET":
		return p.unsupported(tok, "OFFSET is not supported; use LIMIT.")
	case "FETCH":
		return p.unsupported(tok, "FETCH is not supported; use LIMIT.")
	case "INTO":
		return p.unsupported(tok, "SELECT INTO is not supported.")
	case "FOR":
		return p.unsupported(tok, "Locking clauses are not supported.")
	}

	return nil
}

func numericValue(tok token) (catalog.Value, error) {
	if tok.kind == tokenInteger {
		value, err := strconv.ParseInt(tok.text, 10, 64)
		if err != nil {
			return catalog.Null(), errorAt(CodeParseError, tok.line, tok.column,
				"Integer literal %s is out of range for a 64-bit integer.", tok.text)
		}
		return catalog.Int(value), nil
	}

	value, err := strconv.ParseFloat(tok.text, 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		return catalog.Null(), errorAt(CodeParseError, tok.line, tok.column,
			"Decimal literal %s is out of range.", tok.text)
	}
	return catalog.Decimal(value), nil
}

// maxLimitValue is the largest value an int can hold on this platform.
func maxLimitValue() int64 { return int64(^uint(0) >> 1) }

func comparisonOp(kind tokenKind) (BinaryOp, bool) {
	switch kind {
	case tokenEqual:
		return OpEqual, true
	case tokenNotEqual:
		return OpNotEqual, true
	case tokenLess:
		return OpLess, true
	case tokenLessEqual:
		return OpLessEqual, true
	case tokenGreater:
		return OpGreater, true
	case tokenGreaterEqual:
		return OpGreaterEqual, true
	default:
		return 0, false
	}
}

func (p *parser) current() token { return p.peek(0) }

func (p *parser) peek(offset int) token {
	index := p.index + offset
	if index >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[index]
}

func (p *parser) peekIs(offset int, kind tokenKind) bool {
	return p.peek(offset).kind == kind
}

func (p *parser) advance() token {
	tok := p.current()
	if tok.kind != tokenEOF {
		p.index++
	}
	return tok
}

func (p *parser) match(kind tokenKind) bool {
	if !p.peekIs(0, kind) {
		return false
	}
	p.advance()
	return true
}

func (p *parser) checkKeyword(word string) bool {
	tok := p.current()
	return tok.kind == tokenIdentifier && !tok.quoted && strings.EqualFold(tok.text, word)
}

func (p *parser) matchKeyword(word string) bool {
	if !p.checkKeyword(word) {
		return false
	}
	p.advance()
	return true
}

func (p *parser) expect(kind tokenKind, what string) (token, error) {
	tok := p.current()
	if tok.kind != kind {
		return token{}, errorAt(CodeParseError, tok.line, tok.column,
			"Expected %s, found %s.", what, tok.describe())
	}
	return p.advance(), nil
}

func (p *parser) expectKeyword(word string) (token, error) {
	if !p.checkKeyword(word) {
		tok := p.current()
		return token{}, errorAt(CodeParseError, tok.line, tok.column,
			"Expected %s, found %s.", word, tok.describe())
	}
	return p.advance(), nil
}

func (p *parser) unsupported(tok token, format string, args ...any) error {
	return errorAt(CodeUnsupportedSyntax, tok.line, tok.column, format, args...)
}

func (p *parser) unexpected(tok token) error {
	if tok.kind == tokenEOF {
		return errorAt(CodeParseError, tok.line, tok.column, "Unexpected end of input.")
	}
	return errorAt(CodeParseError, tok.line, tok.column, "Unexpected %s.", tok.describe())
}
