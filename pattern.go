package plexmatch

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// TokenKind is the kind of number a pattern's match token extracts.
type TokenKind int

const (
	// TokenSeason matches a season number: "s", "se" or "season".
	TokenSeason TokenKind = iota + 1
	// TokenEpisode matches an episode number: "e", "ep" or "episode".
	TokenEpisode
	// TokenSpecial matches a special number: "sp" or "special".
	TokenSpecial
	// TokenSeasonEpisode matches a combined season and episode number, as in
	// "214" for season 2 episode 14: "see".
	TokenSeasonEpisode
)

// String returns the canonical name of the token kind.
func (k TokenKind) String() string {
	switch k {
	case TokenSeason:
		return "season"
	case TokenEpisode:
		return "episode"
	case TokenSpecial:
		return "special"
	case TokenSeasonEpisode:
		return "season-episode"
	}
	return fmt.Sprintf("TokenKind(%d)", int(k))
}

// tokenKinds maps every accepted spelling of a match token name to its kind.
var tokenKinds = map[string]TokenKind{
	"s": TokenSeason, "se": TokenSeason, "season": TokenSeason,
	"e": TokenEpisode, "ep": TokenEpisode, "episode": TokenEpisode,
	"sp": TokenSpecial, "special": TokenSpecial,
	"see": TokenSeasonEpisode,
}

// Token is a {…} match token inside a pattern. A token matches a number in a
// file name and may adjust it:
//
//	{ep}            the episode number
//	{sp,1-3,+4}     specials 1 to 3, renumbered from 5
//	{se,l2}         a season number of at most two digits
type Token struct {
	// Kind is the number the token matches.
	Kind TokenKind

	// Name is the token name as it was written, for example "ep" or "season".
	Name string

	// Min and Max bound the values the token accepts, both inclusive. They
	// are 0 when the token carries no range.
	Min, Max int

	// Offset is added to a matched value, so that {sp,1-3,+4} reads a file
	// numbered 5 to 7 as specials 1 to 3. It is 0 when the token carries no
	// offset.
	Offset int

	// MaxLength limits how many digits the token matches. It is 0 when the
	// token carries no limit.
	MaxLength int
}

// HasRange reports whether the token restricts the values it accepts.
func (t Token) HasRange() bool { return t.Min != 0 || t.Max != 0 }

// Pattern is a "pattern" or "pt" hint: a glob that matches several files at
// once and extracts season and episode numbers from their names.
//
// The pattern is a path using forward slashes, in which "*" matches a run of
// characters and match tokens in curly braces capture numbers. A backslash
// escapes the character after it, so "\{", "\}", "\*" and "\\" stand for the
// characters themselves.
//
//	Bonus/Bonus {sp,1-3,+4}.mp4
//	Show Part {se} - {ep} - *
//
// Path holds the pattern itself and is what Marshal writes, so it is the
// authoritative value. Tokens holds the match tokens ParsePattern found in it
// and is informational; it is empty for a Pattern built by hand.
type Pattern struct {
	// Path is the pattern as written, without the surrounding whitespace.
	Path string

	// Tokens are the match tokens of Path, in the order they appear. Use
	// ParsePattern to fill it.
	Tokens []Token
}

// String returns the pattern's path.
func (p Pattern) String() string { return p.Path }

// ParsePattern parses a pattern hint value. It reports an error when the
// pattern is empty, when a match token is malformed or named something
// unknown, or when the pattern contains no token that captures an episode
// number, which Plex requires of every pattern.
func ParsePattern(value string) (Pattern, error) {
	p := Pattern{Path: trimSpace(value)}
	if p.Path == "" {
		return Pattern{}, errors.New("empty pattern")
	}
	tokens, err := scanTokens(p.Path)
	if err != nil {
		return Pattern{}, err
	}
	episodes := 0
	for _, t := range tokens {
		switch t.Kind {
		case TokenEpisode, TokenSpecial, TokenSeasonEpisode:
			episodes++
		}
	}
	if episodes == 0 {
		return Pattern{}, errors.New("pattern has no episode number token")
	}
	p.Tokens = tokens
	return p, nil
}

// scanTokens collects the match tokens of a pattern.
func scanTokens(s string) ([]Token, error) {
	var tokens []Token
	for i := 0; i < len(s); {
		switch s[i] {
		case '\\':
			// The escaped character is literal, whatever it is.
			i += 2
		case '{':
			token, next, err := scanToken(s, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token)
			i = next
		default:
			i++
		}
	}
	return tokens, nil
}

// scanToken reads the match token that starts at the brace at index start, and
// reports the index just past it.
func scanToken(s string, start int) (Token, int, error) {
	i := start + 1
	for {
		switch {
		case i >= len(s):
			return Token{}, 0, errors.New("unterminated match token")
		case s[i] == '\\':
			i += 2
			continue
		case s[i] == '}':
			token, err := parseToken(s[start+1 : i])
			if err != nil {
				return Token{}, 0, err
			}
			return token, i + 1, nil
		case s[i] == '{':
			return Token{}, 0, errors.New("match token inside a match token")
		}
		i++
	}
}

// parseToken reads the contents of a match token: a name and, after commas,
// its parameters.
func parseToken(content string) (Token, error) {
	fields := strings.Split(content, ",")
	name := trimSpace(fields[0])
	kind, ok := tokenKinds[asciiLower(name)]
	if !ok {
		return Token{}, fmt.Errorf("unknown match token %q", name)
	}
	token := Token{Kind: kind, Name: name}
	for _, field := range fields[1:] {
		field = trimSpace(field)
		switch {
		case field == "":
			return Token{}, errors.New("empty match token parameter")
		case field[0] == '+' || field[0] == '-':
			offset, err := strconv.Atoi(field)
			if err != nil {
				return Token{}, fmt.Errorf("invalid match token offset %q", field)
			}
			token.Offset = offset
		case field[0] == 'l' || field[0] == 'L':
			length, err := strconv.Atoi(field[1:])
			if err != nil || length <= 0 {
				return Token{}, fmt.Errorf("invalid match token length %q", field)
			}
			token.MaxLength = length
		default:
			first, last, ok := strings.Cut(field, "-")
			if !ok {
				return Token{}, fmt.Errorf("invalid match token parameter %q", field)
			}
			min, err := strconv.Atoi(trimSpace(first))
			if err != nil {
				return Token{}, fmt.Errorf("invalid match token range %q", field)
			}
			max, err := strconv.Atoi(trimSpace(last))
			if err != nil {
				return Token{}, fmt.Errorf("invalid match token range %q", field)
			}
			if max < min {
				return Token{}, fmt.Errorf("match token range %q is not ascending", field)
			}
			token.Min, token.Max = min, max
		}
	}
	return token, nil
}

func (p Pattern) validate() error {
	if _, err := ParsePattern(p.Path); err != nil {
		return err
	}
	return checkLine("pattern", p.Path)
}
