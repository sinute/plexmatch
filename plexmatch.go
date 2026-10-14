// Package plexmatch reads and writes Plex ".plexmatch" match-hint files.
//
// A ".plexmatch" file tells Plex Media Server how to match a TV series without
// renaming its files. It lives in a series directory, or in a season directory,
// and holds one hint per line:
//
//	# Lines that start with a hash are comments.
//	title: Cool Show
//	year: 2018
//	season: 2
//	ep: 01: A Tricky File Name.mkv
//	pattern: Bonus/Bonus {sp,1-3,+4}.mp4
//
// Whitespace around a line, a hint name and a hint value is ignored, and hint
// names are case-insensitive. Everything after the colon belongs to the value,
// so a file name keeps its own colons and spaces.
//
// Unmarshal reads a file into a PlexMatch and Marshal writes one back out. A
// directive whose name this package does not know is kept in
// PlexMatch.UnknownHints, so that reading and writing a file does not discard
// hints added by a newer Plex Media Server.
//
// See https://support.plex.tv/articles/plexmatch/ for the format.
package plexmatch

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// PlexMatch holds the hints of one .plexmatch file.
//
// A hint that is repeated in one file keeps the value of its first occurrence.
type PlexMatch struct {
	// Title is the title the series matches against. Both "title" and its
	// synonym "show" set it.
	Title string

	// Year is the release year of the series ("year").
	Year int

	// Season is the season number of the directory ("season").
	Season int

	// TMDBID, TVDBID and IMDBID are the series' numeric IDs at The Movie
	// Database, TheTVDB and IMDb ("tmdbid", "tvdbid", "imdbid"). An "imdbid"
	// hint may carry the "tt" prefix; only the number is kept.
	TMDBID int
	TVDBID int
	IMDBID int

	// GUID is the Plex URI-style identification from the "guid" hint, in the
	// form [source]://[id], for example "plex://show/5d9c07fd02391c001f57e02f".
	GUID string

	// Episodes holds the "ep" and "episode" hints, in the order they were
	// read.
	Episodes []Episode

	// Patterns holds the "pattern" and "pt" hints, in the order they were
	// read. Plex uses the first pattern that matches a file, so the order
	// matters.
	Patterns []Pattern

	// UnknownHints holds the directives whose names this package does not
	// know, in the order they were read, so that they survive a round trip.
	UnknownHints []UnknownHint
}

// UnknownHint is a directive whose name this package does not recognise.
//
// Plex Media Server has added hint names over time, so a directive that is not
// known here is kept rather than treated as an error.
type UnknownHint struct {
	// Name is the hint name as it was written.
	Name string

	// Value is the hint value with the surrounding whitespace removed.
	Value string
}

// bom is the UTF-8 byte order mark that may open a .plexmatch file.
var bom = string([]byte{0xef, 0xbb, 0xbf})

// SyntaxError reports a line of a .plexmatch file that could not be read.
type SyntaxError struct {
	// Line is the 1-based number of the offending line.
	Line int

	// Text is the offending line, without the surrounding whitespace.
	Text string

	// Msg describes what is wrong with the line.
	Msg string
}

// Error implements the error interface.
func (e *SyntaxError) Error() string {
	return fmt.Sprintf("plexmatch: line %d: %s: %q", e.Line, e.Msg, e.Text)
}

// Unmarshal reads the contents of a .plexmatch file.
//
// Reading stops at the first line that cannot be read; the hints gathered
// before it are returned together with a *SyntaxError that names the line.
func Unmarshal(data []byte) (PlexMatch, error) {
	var p PlexMatch
	for i, raw := range strings.Split(normalizeLineEndings(string(data)), "\n") {
		if i == 0 {
			// A byte order mark is not part of the first hint name.
			raw = strings.TrimPrefix(raw, bom)
		}
		line := trimSpace(raw)
		if line == "" || line[0] == '#' {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return p, &SyntaxError{Line: i + 1, Text: line, Msg: "missing ':' after the hint name"}
		}
		name, value = trimSpace(name), trimSpace(value)
		if name == "" {
			return p, &SyntaxError{Line: i + 1, Text: line, Msg: "missing hint name"}
		}
		if err := p.apply(name, value); err != nil {
			return p, &SyntaxError{Line: i + 1, Text: line, Msg: err.Error()}
		}
	}
	return p, nil
}

// apply records one hint. The name is the hint name as written.
func (p *PlexMatch) apply(name, value string) error {
	switch asciiLower(name) {
	case "title", "show":
		if value == "" {
			return errors.New("missing value")
		}
		if p.Title == "" {
			p.Title = value
		}
	case "year":
		year, err := parseCount(value)
		if err != nil {
			return err
		}
		if p.Year == 0 {
			p.Year = year
		}
	case "season":
		season, err := parseCount(value)
		if err != nil {
			return err
		}
		if p.Season == 0 {
			p.Season = season
		}
	case "tmdbid":
		id, err := parseCount(value)
		if err != nil {
			return err
		}
		if p.TMDBID == 0 {
			p.TMDBID = id
		}
	case "tvdbid":
		id, err := parseCount(value)
		if err != nil {
			return err
		}
		if p.TVDBID == 0 {
			p.TVDBID = id
		}
	case "imdbid":
		// IMDb IDs are written both as "tt1234567" and as "1234567".
		id, err := parseCount(strings.TrimPrefix(asciiLower(value), "tt"))
		if err != nil {
			return err
		}
		if p.IMDBID == 0 {
			p.IMDBID = id
		}
	case "guid":
		if value == "" {
			return errors.New("missing value")
		}
		if p.GUID == "" {
			p.GUID = value
		}
	case "ep", "episode":
		episode, err := parseEpisode(value)
		if err != nil {
			return err
		}
		p.Episodes = append(p.Episodes, episode)
	case "pattern", "pt":
		pattern, err := ParsePattern(value)
		if err != nil {
			return err
		}
		p.Patterns = append(p.Patterns, pattern)
	default:
		p.UnknownHints = append(p.UnknownHints, UnknownHint{Name: name, Value: value})
	}
	return nil
}

// Marshal renders p as the contents of a .plexmatch file.
//
// The episodes are written sorted by season and episode number, with specials
// first. Every hint is written: use Unique to drop hints that name an episode
// an earlier hint already claims, and Validate to check the values against the
// constraints Plex places on them.
func Marshal(p PlexMatch) []byte {
	var buf bytes.Buffer
	scalar := func(name, value string) {
		if value != "" {
			fmt.Fprintf(&buf, "%s: %s\n\n", name, value)
		}
	}
	count := func(name string, value int) {
		if value != 0 {
			fmt.Fprintf(&buf, "%s: %d\n\n", name, value)
		}
	}
	scalar("title", p.Title)
	count("year", p.Year)
	count("season", p.Season)
	count("tmdbid", p.TMDBID)
	count("tvdbid", p.TVDBID)
	count("imdbid", p.IMDBID)
	scalar("guid", p.GUID)

	blank := func() {
		if buf.Len() > 0 && !bytes.HasSuffix(buf.Bytes(), []byte("\n\n")) {
			buf.WriteByte('\n')
		}
	}
	if len(p.Episodes) > 0 {
		blank()
		for _, e := range sortedEpisodes(p.Episodes) {
			fmt.Fprintf(&buf, "ep: %s\n", e)
		}
	}
	if len(p.Patterns) > 0 {
		blank()
		for _, pattern := range p.Patterns {
			fmt.Fprintf(&buf, "pattern: %s\n", pattern.Path)
		}
	}
	if len(p.UnknownHints) > 0 {
		blank()
		for _, hint := range p.UnknownHints {
			fmt.Fprintf(&buf, "%s: %s\n", hint.Name, hint.Value)
		}
	}
	return buf.Bytes()
}

// Merge returns p with the hints of overlay applied on top of it: a hint that
// overlay sets wins over the one p sets. Episodes, patterns and unrecognised
// hints are appended with p's own first.
//
// It combines the .plexmatch file of a series directory with the one of a
// season directory beneath it.
func (p PlexMatch) Merge(overlay PlexMatch) PlexMatch {
	merged := p
	if overlay.Title != "" {
		merged.Title = overlay.Title
	}
	if overlay.Year != 0 {
		merged.Year = overlay.Year
	}
	if overlay.Season != 0 {
		merged.Season = overlay.Season
	}
	if overlay.TMDBID != 0 {
		merged.TMDBID = overlay.TMDBID
	}
	if overlay.TVDBID != 0 {
		merged.TVDBID = overlay.TVDBID
	}
	if overlay.IMDBID != 0 {
		merged.IMDBID = overlay.IMDBID
	}
	if overlay.GUID != "" {
		merged.GUID = overlay.GUID
	}
	merged.Episodes = append(append([]Episode(nil), p.Episodes...), overlay.Episodes...)
	merged.Patterns = append(append([]Pattern(nil), p.Patterns...), overlay.Patterns...)
	merged.UnknownHints = append(append([]UnknownHint(nil), p.UnknownHints...), overlay.UnknownHints...)
	return merged
}

// Unique returns a copy of p whose episode hints are sorted by season and
// episode number, with the first hint that names an episode or range kept and
// any later hint naming the same one dropped.
//
// Plex matches one file to one episode, so hints gathered from a library may
// claim the same episode several times. Marshal writes every hint it is given,
// so call Unique first when only one of them should be kept.
func (p PlexMatch) Unique() PlexMatch {
	if len(p.Episodes) == 0 {
		return p
	}
	unique := p
	unique.Episodes = uniqueEpisodes(p.Episodes)
	return unique
}

// Validate reports whether p can be written as a .plexmatch file that Plex
// would accept. It checks the values that Marshal writes without looking at
// them, and is meant for a PlexMatch that was built in Go rather than read
// from a file.
func (p PlexMatch) Validate() error {
	for _, field := range []struct {
		name  string
		value int
	}{
		{"year", p.Year},
		{"season", p.Season},
		{"tmdbid", p.TMDBID},
		{"tvdbid", p.TVDBID},
		{"imdbid", p.IMDBID},
	} {
		if field.value < 0 {
			return fmt.Errorf("plexmatch: %s is negative: %d", field.name, field.value)
		}
	}
	if err := checkLine("title", p.Title); err != nil {
		return err
	}
	if err := checkLine("guid", p.GUID); err != nil {
		return err
	}
	for i, episode := range p.Episodes {
		if err := episode.validate(); err != nil {
			return fmt.Errorf("plexmatch: episode %d: %w", i+1, err)
		}
	}
	for i, pattern := range p.Patterns {
		if err := pattern.validate(); err != nil {
			return fmt.Errorf("plexmatch: pattern %d: %w", i+1, err)
		}
	}
	for i, hint := range p.UnknownHints {
		if hint.Name == "" || strings.ContainsAny(hint.Name, ":\r\n") {
			return fmt.Errorf("plexmatch: hint %d: %q is not a hint name", i+1, hint.Name)
		}
		if err := checkLine("hint value", hint.Value); err != nil {
			return fmt.Errorf("plexmatch: hint %d: %w", i+1, err)
		}
	}
	return nil
}

// checkLine reports whether a hint value fits on the line that carries it.
func checkLine(what, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("plexmatch: %s contains a line break: %q", what, value)
	}
	return nil
}

// sortedEpisodes returns a copy of episodes in the order Marshal writes them.
func sortedEpisodes(episodes []Episode) []Episode {
	if len(episodes) == 0 {
		return nil
	}
	sorted := make([]Episode, len(episodes))
	copy(sorted, episodes)
	sort.SliceStable(sorted, func(i, j int) bool {
		return compareEpisodes(sorted[i], sorted[j]) < 0
	})
	return sorted
}

// uniqueEpisodes returns the episodes sorted, with the first hint naming an
// episode or range kept and any later hint naming the same one dropped.
func uniqueEpisodes(episodes []Episode) []Episode {
	if len(episodes) == 0 {
		return nil
	}
	unique := make([]Episode, 0, len(episodes))
	for _, episode := range sortedEpisodes(episodes) {
		if len(unique) > 0 && sameTarget(unique[len(unique)-1], episode) {
			continue
		}
		unique = append(unique, episode)
	}
	return unique
}

// compareEpisodes orders episodes by the episode they name, with specials
// first. Episodes that name the same episode compare equal, so that a stable
// sort keeps them in the order they were read.
func compareEpisodes(a, b Episode) int {
	if a.Special != b.Special {
		if a.Special {
			return -1
		}
		return 1
	}
	for _, field := range [][2]int{
		{a.Season, b.Season},
		{a.Episode, b.Episode},
		{a.LastEpisode, b.LastEpisode},
		{a.LastSeason, b.LastSeason},
	} {
		if field[0] != field[1] {
			if field[0] < field[1] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// sameTarget reports whether two episodes name the same number or range,
// whatever file they point at.
func sameTarget(a, b Episode) bool {
	a.Name, b.Name = "", ""
	return a == b
}

// parseCount reads a non-negative decimal number.
func parseCount(value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid number %q", value)
	}
	return n, nil
}

// normalizeLineEndings turns the CRLF of a Windows file, and a lone CR, into
// the LF that separates lines, so that a value cannot hold a stray carriage
// return and the reported line numbers match what an editor shows.
func normalizeLineEndings(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// trimSpace removes the ASCII whitespace that may surround a line, a hint name
// or a hint value. Only ASCII whitespace is removed, because other space
// characters may belong to a file name.
func trimSpace(s string) string {
	return strings.Trim(s, " \t\r\v\f")
}

// lower folds ASCII upper case letters to lower case without changing the
// length of the string, so that byte offsets into it stay valid.
func asciiLower(s string) string {
	for i := 0; i < len(s); i++ {
		if 'A' <= s[i] && s[i] <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if 'A' <= b[j] && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}
