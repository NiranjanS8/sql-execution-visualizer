package sqlparse

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// eof is returned by the lexer when input is exhausted. It is distinct from the
// NUL rune, which is rejected as an unexpected character.
const eof rune = -1

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenIdentifier
	tokenInteger
	tokenDecimal
	tokenString
	tokenComma
	tokenDot
	tokenStar
	tokenLParen
	tokenRParen
	tokenSemicolon
	tokenPlus
	tokenMinus
	tokenSlash
	tokenEqual
	tokenNotEqual
	tokenLess
	tokenLessEqual
	tokenGreater
	tokenGreaterEqual
)

// token is one lexical unit. Text holds the raw spelling for punctuation and
// numbers, the decoded value for strings, and the decoded name for identifiers.
// Quoted distinguishes "from" from the keyword FROM.
type token struct {
	kind   tokenKind
	text   string
	quoted bool
	line   int
	column int
}

func (t token) position() Position { return Position{Line: t.line, Column: t.column} }

func (t token) describe() string {
	switch t.kind {
	case tokenEOF:
		return "end of input"
	case tokenString:
		return "string literal"
	case tokenInteger, tokenDecimal:
		return "number " + t.text
	default:
		return fmt.Sprintf("%q", t.text)
	}
}

type lexer struct {
	src    string
	offset int
	line   int
	column int
}

func lex(src string) ([]token, error) {
	l := &lexer{src: src, line: 1, column: 1}
	tokens := make([]token, 0, 16)

	for {
		if err := l.skipTrivia(); err != nil {
			return nil, err
		}

		line, column := l.line, l.column
		r := l.peek()
		if r == eof {
			tokens = append(tokens, token{kind: tokenEOF, line: line, column: column})
			return tokens, nil
		}

		switch {
		case isIdentifierStart(r):
			tokens = append(tokens, l.scanIdentifier())
		case isDigit(r):
			tok, err := l.scanNumber()
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
		case r == '\'':
			tok, err := l.scanString()
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
		case r == '"':
			tok, err := l.scanQuotedIdentifier()
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
		case r == '`':
			return nil, errorAt(CodeUnsupportedSyntax, line, column,
				"Backtick identifiers are not supported; use double quotes.")
		default:
			tok, err := l.scanPunctuation(r, line, column)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
		}
	}
}

func (l *lexer) scanIdentifier() token {
	line, column := l.line, l.column
	start := l.offset
	for isIdentifierPart(l.peek()) {
		l.next()
	}
	return token{kind: tokenIdentifier, text: l.src[start:l.offset], line: line, column: column}
}

func (l *lexer) scanNumber() (token, error) {
	line, column := l.line, l.column
	start := l.offset
	kind := tokenInteger

	for isDigit(l.peek()) {
		l.next()
	}
	if l.peek() == '.' {
		kind = tokenDecimal
		l.next()
		for isDigit(l.peek()) {
			l.next()
		}
	}
	if l.peek() == 'e' || l.peek() == 'E' {
		next := l.peekNext()
		if isDigit(next) || ((next == '+' || next == '-') && isDigit(l.peekAhead(2))) {
			kind = tokenDecimal
			l.next()
			if l.peek() == '+' || l.peek() == '-' {
				l.next()
			}
			for isDigit(l.peek()) {
				l.next()
			}
		}
	}

	return token{kind: kind, text: l.src[start:l.offset], line: line, column: column}, nil
}

func (l *lexer) scanString() (token, error) {
	line, column := l.line, l.column
	l.next() // opening quote

	var value strings.Builder
	for {
		r := l.peek()
		switch r {
		case eof:
			return token{}, errorAt(CodeParseError, line, column, "Unterminated string literal.")
		case '\'':
			l.next()
			if l.peek() == '\'' {
				l.next()
				value.WriteRune('\'')
				continue
			}
			return token{kind: tokenString, text: value.String(), line: line, column: column}, nil
		default:
			l.next()
			value.WriteRune(r)
		}
	}
}

func (l *lexer) scanQuotedIdentifier() (token, error) {
	line, column := l.line, l.column
	l.next() // opening quote

	var name strings.Builder
	for {
		r := l.peek()
		switch r {
		case eof:
			return token{}, errorAt(CodeParseError, line, column, "Unterminated quoted identifier.")
		case '"':
			l.next()
			if l.peek() == '"' {
				l.next()
				name.WriteRune('"')
				continue
			}
			if name.Len() == 0 {
				return token{}, errorAt(CodeParseError, line, column, "Quoted identifier must not be empty.")
			}
			return token{kind: tokenIdentifier, text: name.String(), quoted: true, line: line, column: column}, nil
		default:
			l.next()
			name.WriteRune(r)
		}
	}
}

func (l *lexer) scanPunctuation(r rune, line, column int) (token, error) {
	l.next()

	switch r {
	case ',':
		return token{kind: tokenComma, text: ",", line: line, column: column}, nil
	case '.':
		return token{kind: tokenDot, text: ".", line: line, column: column}, nil
	case '*':
		return token{kind: tokenStar, text: "*", line: line, column: column}, nil
	case '(':
		return token{kind: tokenLParen, text: "(", line: line, column: column}, nil
	case ')':
		return token{kind: tokenRParen, text: ")", line: line, column: column}, nil
	case ';':
		return token{kind: tokenSemicolon, text: ";", line: line, column: column}, nil
	case '+':
		return token{kind: tokenPlus, text: "+", line: line, column: column}, nil
	case '-':
		return token{kind: tokenMinus, text: "-", line: line, column: column}, nil
	case '/':
		return token{kind: tokenSlash, text: "/", line: line, column: column}, nil
	case '=':
		return token{kind: tokenEqual, text: "=", line: line, column: column}, nil
	case '<':
		if l.peek() == '=' {
			l.next()
			return token{kind: tokenLessEqual, text: "<=", line: line, column: column}, nil
		}
		if l.peek() == '>' {
			l.next()
			return token{kind: tokenNotEqual, text: "<>", line: line, column: column}, nil
		}
		return token{kind: tokenLess, text: "<", line: line, column: column}, nil
	case '>':
		if l.peek() == '=' {
			l.next()
			return token{kind: tokenGreaterEqual, text: ">=", line: line, column: column}, nil
		}
		return token{kind: tokenGreater, text: ">", line: line, column: column}, nil
	case '!':
		if l.peek() == '=' {
			l.next()
			return token{kind: tokenNotEqual, text: "!=", line: line, column: column}, nil
		}
		return token{}, errorAt(CodeParseError, line, column, `Unexpected "!"; did you mean "!="?`)
	case ':':
		if l.peek() == ':' {
			l.next()
			return token{}, errorAt(CodeUnsupportedSyntax, line, column,
				"PostgreSQL-style casts (::) are not supported.")
		}
		return token{}, errorAt(CodeParseError, line, column, `Unexpected ":"`)
	default:
		return token{}, errorAt(CodeParseError, line, column, "Unexpected character %q.", r)
	}
}

// skipTrivia consumes whitespace, line comments, and block comments.
func (l *lexer) skipTrivia() error {
	for {
		r := l.peek()
		switch {
		case r == ' ' || r == '\t' || r == '\r' || r == '\n':
			l.next()
		case r == '-' && l.peekNext() == '-':
			for l.peek() != eof && l.peek() != '\n' {
				l.next()
			}
		case r == '/' && l.peekNext() == '*':
			line, column := l.line, l.column
			l.next()
			l.next()
			closed := false
			for l.peek() != eof {
				if l.peek() == '*' && l.peekNext() == '/' {
					l.next()
					l.next()
					closed = true
					break
				}
				l.next()
			}
			if !closed {
				return errorAt(CodeParseError, line, column, "Unterminated block comment.")
			}
		default:
			return nil
		}
	}
}

func (l *lexer) peek() rune {
	if l.offset >= len(l.src) {
		return eof
	}
	r, _ := utf8.DecodeRuneInString(l.src[l.offset:])
	return r
}

func (l *lexer) peekNext() rune { return l.peekAhead(1) }

// peekAhead returns the rune n positions ahead of the cursor, or eof.
func (l *lexer) peekAhead(n int) rune {
	offset := l.offset
	for i := 0; i < n; i++ {
		if offset >= len(l.src) {
			return eof
		}
		_, size := utf8.DecodeRuneInString(l.src[offset:])
		offset += size
	}
	if offset >= len(l.src) {
		return eof
	}
	r, _ := utf8.DecodeRuneInString(l.src[offset:])
	return r
}

func (l *lexer) next() rune {
	if l.offset >= len(l.src) {
		return eof
	}
	r, size := utf8.DecodeRuneInString(l.src[l.offset:])
	l.offset += size
	if r == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	return r
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func isIdentifierStart(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isIdentifierPart(r rune) bool { return isIdentifierStart(r) || isDigit(r) }
