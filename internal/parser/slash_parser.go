package parser

import (
	"strings"
	"unicode"
)

// TokenKind describes what a lexed token represents.
type TokenKind int

const (
	TokenSlash   TokenKind = iota // the leading '/'
	TokenWord                     // a command segment or argument word
	TokenString                   // a quoted string argument
	TokenEOF                      // end of input
)

// Token is a single lexed element from slash input.
type Token struct {
	Kind  TokenKind
	Value string
}

// ParseResult holds the structured output of parsing a slash command input.
type ParseResult struct {
	// IsSlash is true when the input begins with '/'.
	IsSlash bool

	// CommandPath is the canonical hierarchical path rebuilt from command segments.
	// E.g. "/profile list" → "profile list".
	CommandPath string

	// Args are the positional arguments after the command path.
	Args []string

	// RawInput is the original unmodified input string.
	RawInput string

	// Partial is true when the input looks like an incomplete slash command
	// (no non-slash words yet, or only partial words typed).
	Partial bool
}

// Parse lexes and parses a slash command input string.
// It handles:
//   - Plain text (no leading '/') → IsSlash=false
//   - Hierarchical slash commands  → IsSlash=true, CommandPath and Args populated
//   - Quoted arguments (single or double quotes)
//   - Partial inputs (for suggestion mode)
func Parse(input string) ParseResult {
	result := ParseResult{RawInput: input}
	if !strings.HasPrefix(input, "/") {
		return result
	}
	result.IsSlash = true

	tokens := lex(input[1:]) // skip the leading '/'

	// Determine command path segments (TokenWord before any TokenString or
	// before any segment that looks like an argument value).
	var pathSegments []string
	var args []string

	// Canonical command syntax: "/group action [args...]"
	// The first two word tokens form the command path; everything after is an argument.
	const maxPathSegments = 2
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind == TokenEOF {
			break
		}
		if tok.Kind == TokenSlash {
			i++
			continue
		}
		if tok.Kind == TokenString {
			// Quoted strings are always arguments.
			args = append(args, tok.Value)
			i++
			continue
		}
		// TokenWord: first maxPathSegments words are the path, rest are args.
		if len(pathSegments) < maxPathSegments && len(args) == 0 {
			pathSegments = append(pathSegments, tok.Value)
		} else {
			args = append(args, tok.Value)
		}
		i++
	}

	result.CommandPath = strings.Join(pathSegments, " ")
	result.Args = args

	// Mark as partial if no words were found yet (only '/' typed) or if the
	// input ends with a space after the slash (suggesting mid-input state).
	trimmed := strings.TrimSpace(input[1:])
	result.Partial = trimmed == "" || strings.HasSuffix(input, " ")

	return result
}

// lex tokenises the input string (after the leading '/') into tokens.
func lex(input string) []Token {
	var tokens []Token
	runes := []rune(input)
	i := 0

	for i < len(runes) {
		ch := runes[i]

		// Skip whitespace between tokens.
		if unicode.IsSpace(ch) {
			i++
			continue
		}

		// Nested slash (e.g. future subcommand syntax) – treat as delimiter.
		if ch == '/' {
			tokens = append(tokens, Token{Kind: TokenSlash, Value: "/"})
			i++
			continue
		}

		// Quoted string.
		if ch == '"' || ch == '\'' {
			quote := ch
			i++
			start := i
			for i < len(runes) && runes[i] != quote {
				if runes[i] == '\\' {
					i++ // skip escaped character
				}
				i++
			}
			value := string(runes[start:i])
			if i < len(runes) {
				i++ // consume closing quote
			}
			tokens = append(tokens, Token{Kind: TokenString, Value: value})
			continue
		}

		// Word token (runs of non-whitespace, non-quote, non-slash characters).
		start := i
		for i < len(runes) && !unicode.IsSpace(runes[i]) && runes[i] != '"' && runes[i] != '\'' && runes[i] != '/' {
			i++
		}
		tokens = append(tokens, Token{Kind: TokenWord, Value: string(runes[start:i])})
	}

	tokens = append(tokens, Token{Kind: TokenEOF})
	return tokens
}

// IsSlashInput returns true if the input string begins with '/'.
func IsSlashInput(input string) bool {
	return strings.HasPrefix(input, "/")
}

// PrefixFromInput extracts the slash command prefix typed so far, suitable for suggestion lookup.
// E.g. "/pro" → "pro", "/profile li" → "profile li", "/" → "".
func PrefixFromInput(input string) string {
	if !strings.HasPrefix(input, "/") {
		return ""
	}
	inner := input[1:]
	// If ends with space, the prefix is the full inner string (trimmed).
	return strings.TrimSpace(inner)
}
