package planning

import (
	"strconv"
	"strings"
	"unicode"
)

// Parser tokens.
const (
	TokenEOF     = "EOF"
	TokenID      = "ID"
	TokenRange   = "RANGE"
	TokenAnd     = "AND"
	TokenOr      = "OR"
	TokenLParen  = "LPAREN"
	TokenRParen  = "RPAREN"
	TokenMinus   = "MINUS"
	TokenUnknown = "UNKNOWN"
)

// Token represents a lexical token.
type Token struct {
	Type  string
	Value string
	Pos   int
}

// Lexer tokenizes selector input strings.
type Lexer struct {
	input string
	pos   int
}

// NewLexer creates a new lexer for the given input.
func NewLexer(input string) *Lexer {
	return &Lexer{input: input, pos: 0}
}

// NextToken returns the next token from the input.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	if l.pos >= len(l.input) {
		return Token{Type: TokenEOF, Value: "", Pos: l.pos}
	}

	startPos := l.pos
	ch := l.peek()

	// Check for parentheses
	if ch == '(' {
		l.pos++
		return Token{Type: TokenLParen, Value: "(", Pos: startPos}
	}
	if ch == ')' {
		l.pos++
		return Token{Type: TokenRParen, Value: ")", Pos: startPos}
	}

	// Check for minus (range separator)
	if ch == '-' {
		l.pos++
		return Token{Type: TokenMinus, Value: "-", Pos: startPos}
	}

	// Check for identifiers (AND, OR, or numbers)
	if isIdentifierStart(ch) {
		return l.readIdentifierOrNumber()
	}

	return Token{Type: TokenUnknown, Value: string(ch), Pos: startPos}
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) && unicode.IsSpace(rune(l.input[l.pos])) {
		l.pos++
	}
}

func (l *Lexer) peek() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) readIdentifierOrNumber() Token {
	startPos := l.pos
	var sb strings.Builder

	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if isIdentifierChar(ch) || unicode.IsDigit(rune(ch)) {
			sb.WriteByte(ch)
			l.pos++
		} else {
			break
		}
	}

	value := sb.String()
	upper := strings.ToUpper(value)

	// Check for keywords
	if upper == "AND" {
		return Token{Type: TokenAnd, Value: upper, Pos: startPos}
	}
	if upper == "OR" {
		return Token{Type: TokenOr, Value: upper, Pos: startPos}
	}

	// Otherwise it's a number (ID)
	return Token{Type: TokenID, Value: value, Pos: startPos}
}

func isIdentifierStart(ch byte) bool {
	return unicode.IsLetter(rune(ch)) || unicode.IsDigit(rune(ch))
}

func isIdentifierChar(ch byte) bool {
	return unicode.IsLetter(rune(ch)) || ch == '_'
}

// Parser implements a recursive descent parser for selector expressions.
// Grammar:
//
//	expr       → orExpr
//	orExpr     → andExpr (OR andExpr)*
//	andExpr    → term (AND term)*
//	term       → '(' expr ')' | atomic
//	atomic     → ID | RANGE (ID '-' ID)
type Parser struct {
	lexer   *Lexer
	current Token
}

// NewParser creates a new parser for the given input.
func NewParser(input string) *Parser {
	lexer := NewLexer(input)
	p := &Parser{lexer: lexer}
	p.current = lexer.NextToken()
	return p
}

// Parse parses the input and returns a SelectExpr or an error.
func (p *Parser) Parse() (*SelectExpr, error) {
	expr, err := p.parseOrExpr()
	if err != nil {
		return nil, err
	}

	if p.current.Type != TokenEOF {
		return nil, NewSelectorError(ErrSelectorUnexpectedToken,
			"unexpected token %q at position %d", p.current.Value, p.current.Pos)
	}

	return &SelectExpr{Root: expr}, nil
}

// parseOrExpr: orExpr → andExpr (OR andExpr)*
func (p *Parser) parseOrExpr() (SelectNode, error) {
	left, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}

	children := []SelectNode{left}

	for p.current.Type == TokenOr {
		p.advance()
		right, err := p.parseAndExpr()
		if err != nil {
			return nil, err
		}
		children = append(children, right)
	}

	if len(children) == 1 {
		return children[0], nil
	}

	return &SelectOr{Children: children}, nil
}

// parseAndExpr: andExpr → term (AND term)*
func (p *Parser) parseAndExpr() (SelectNode, error) {
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}

	children := []SelectNode{left}

	for p.current.Type == TokenAnd {
		p.advance()
		right, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		children = append(children, right)
	}

	if len(children) == 1 {
		return children[0], nil
	}

	return &SelectAnd{Children: children}, nil
}

// parseTerm: term → '(' expr ')' | atomic
func (p *Parser) parseTerm() (SelectNode, error) {
	if p.current.Type == TokenLParen {
		p.advance()
		expr, err := p.parseOrExpr()
		if err != nil {
			return nil, err
		}
		if p.current.Type != TokenRParen {
			return nil, NewSelectorError(ErrSelectorMismatchedParen,
				"expected ')' at position %d, got %q", p.current.Pos, p.current.Value)
		}
		p.advance()
		return expr, nil
	}

	return p.parseAtomic()
}

// parseAtomic: atomic → ID | RANGE (ID '-' ID)
func (p *Parser) parseAtomic() (SelectNode, error) {
	if p.current.Type != TokenID {
		return nil, NewSelectorError(ErrSelectorInvalidSyntax,
			"expected ID or '(' at position %d, got %q", p.current.Pos, p.current.Value)
	}

	firstID, err := strconv.Atoi(p.current.Value)
	if err != nil {
		return nil, NewSelectorError(ErrSelectorInvalidID,
			"invalid ID %q at position %d", p.current.Value, p.current.Pos)
	}

	p.advance()

	// Check for range: ID '-' ID
	if p.current.Type == TokenMinus {
		p.advance()
		if p.current.Type != TokenID {
			return nil, NewSelectorError(ErrSelectorInvalidSyntax,
				"expected ID after '-' at position %d, got %q", p.current.Pos, p.current.Value)
		}

		secondID, err := strconv.Atoi(p.current.Value)
		if err != nil {
			return nil, NewSelectorError(ErrSelectorInvalidID,
				"invalid ID %q at position %d", p.current.Value, p.current.Pos)
		}

		if firstID > secondID {
			return nil, NewSelectorError(ErrSelectorInvalidRange,
				"range start %d is greater than end %d", firstID, secondID)
		}

		p.advance()
		return &SelectRange{Start: firstID, End: secondID}, nil
	}

	// Single ID - normalize to IDSet with one element
	return &SelectIDSet{IDs: []int{firstID}}, nil
}

func (p *Parser) advance() {
	p.current = p.lexer.NextToken()
}

// Parse parses a selector expression string and returns the AST.
// Returns a *SelectorError on parse failure.
func Parse(input string) (*SelectExpr, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, NewSelectorError(ErrSelectorEmptyExpression, "empty selector expression")
	}

	parser := NewParser(input)
	return parser.Parse()
}

// ParseAndValidate parses a selector expression and validates it against available PR IDs.
// availableIDs should be a sorted slice of valid PR IDs.
// Returns warnings for IDs in the expression that don't exist in the available set.
func ParseAndValidate(input string, availableIDs []int) (*SelectExpr, []string, error) {
	expr, err := Parse(input)
	if err != nil {
		return nil, nil, err
	}

	if len(availableIDs) == 0 {
		return expr, nil, nil
	}

	// Build a set of available IDs for O(1) lookup
	availableSet := make(map[int]struct{}, len(availableIDs))
	for _, id := range availableIDs {
		availableSet[id] = struct{}{}
	}

	// Check all IDs in the expression
	var warnings []string
	exprIDs := expr.AllIDs()

	for _, id := range exprIDs {
		if _, ok := availableSet[id]; !ok {
			warnings = append(warnings, "PR #"+strconv.Itoa(id)+" not found")
		}
	}

	return expr, warnings, nil
}
