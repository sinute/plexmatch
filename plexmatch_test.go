package plexmatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalSeriesHints(t *testing.T) {
	got, err := Unmarshal([]byte(`
		# The second season of a show that lives in its own directory.
		Title: Cool Show

		# title and show are synonyms, so this one is ignored.
		show: Another Show

		Year: 2018
		Season: 2
		TvdbId: 377269
		tmdbid: 203737
		ImdbId: tt11823076
		Guid: plex://show/5d9c07fd02391c001f57e02f

		unknownhint: kept as it was written
		AnotherUnknown: value
	`))
	require.NoError(t, err)
	assert.Equal(t, PlexMatch{
		Title:        "Cool Show",
		Year:         2018,
		Season:       2,
		TMDBID:       203737,
		TVDBID:       377269,
		IMDBID:       11823076,
		GUID:         "plex://show/5d9c07fd02391c001f57e02f",
		UnknownHints: []UnknownHint{{"unknownhint", "kept as it was written"}, {"AnotherUnknown", "value"}},
	}, got)
}

func TestUnmarshalEpisodeHints(t *testing.T) {
	for _, tc := range []struct {
		name  string
		hint  string
		ep    Episode
		again string // the hint value Marshal writes back
	}{
		{
			name:  "bare number",
			hint:  "ep: 12: Episode 12 - Finale Part 2.mkv",
			ep:    Episode{Name: "Episode 12 - Finale Part 2.mkv", Episode: 12},
			again: "12: Episode 12 - Finale Part 2.mkv",
		},
		{
			name:  "episode form",
			hint:  "Episode: E02: An Ordinary Name.mkv",
			ep:    Episode{Name: "An Ordinary Name.mkv", Episode: 2},
			again: "02: An Ordinary Name.mkv",
		},
		{
			name:  "special",
			hint:  "ep: SP04: Some Special.mkv",
			ep:    Episode{Name: "Some Special.mkv", Special: true, Episode: 4},
			again: "SP04: Some Special.mkv",
		},
		{
			name:  "season 0 is a special, however it is spelled",
			hint:  "ep: S00E04: Some Special.mkv",
			ep:    Episode{Name: "Some Special.mkv", Special: true, Episode: 4},
			again: "SP04: Some Special.mkv",
		},
		{
			name:  "season and episode",
			hint:  "ep: S03E04: Season 03/Winter Comes.mkv",
			ep:    Episode{Name: "Season 03/Winter Comes.mkv", Season: 3, Episode: 4},
			again: "S03E04: Season 03/Winter Comes.mkv",
		},
		{
			name:  "range of the current season",
			hint:  "ep: 01-02: A Double Episode.mkv",
			ep:    Episode{Name: "A Double Episode.mkv", Episode: 1, LastEpisode: 2},
			again: "01-02: A Double Episode.mkv",
		},
		{
			name:  "range with an explicit season",
			hint:  "ep: S03E12-S03E13: A Double Episode.mkv",
			ep:    Episode{Name: "A Double Episode.mkv", Season: 3, Episode: 12, LastEpisode: 13},
			again: "S03E12-S03E13: A Double Episode.mkv",
		},
		{
			name:  "range crossing seasons",
			hint:  "ep: S03E12-S04E01: A Double Episode.mkv",
			ep:    Episode{Name: "A Double Episode.mkv", Season: 3, Episode: 12, LastEpisode: 1, LastSeason: 4},
			again: "S03E12-S04E01: A Double Episode.mkv",
		},
		{
			name:  "range of specials",
			hint:  "ep: SP01-SP02: A Double Special.mkv",
			ep:    Episode{Name: "A Double Special.mkv", Special: true, Episode: 1, LastEpisode: 2},
			again: "SP01-SP02: A Double Special.mkv",
		},
		{
			name:  "the file name keeps its own colons",
			hint:  "ep: 01: Show - 01: The Beginning.mkv",
			ep:    Episode{Name: "Show - 01: The Beginning.mkv", Episode: 1},
			again: "01: Show - 01: The Beginning.mkv",
		},
		{
			name:  "unpadded number",
			hint:  "ep: 1: 1_The.Ends.Beginning.mkv",
			ep:    Episode{Name: "1_The.Ends.Beginning.mkv", Episode: 1},
			again: "01: 1_The.Ends.Beginning.mkv",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Unmarshal([]byte(tc.hint))
			require.NoError(t, err)
			require.Len(t, got.Episodes, 1)
			assert.Equal(t, tc.ep, got.Episodes[0], "parsed episode")
			assert.Equal(t, tc.again, got.Episodes[0].String(), "written back")
		})
	}
}

func TestUnmarshalPatternHints(t *testing.T) {
	got, err := Unmarshal([]byte(`
		# Bonus clips 1 through 3 are numbered 5 through 8 on TVDB
		Pattern: Bonus/Bonus {sp,1-3,+4}.mp4
		# Clips 4 through 6 are numbered 10 through 12
		Pattern: Bonus/Bonus {sp,4-6,+6}.mp4
		# These files use an unusual naming convention
		pt: Show Part {se} - {ep} - *
	`))
	require.NoError(t, err)
	require.Len(t, got.Patterns, 3)

	assert.Equal(t, Pattern{
		Path: "Bonus/Bonus {sp,1-3,+4}.mp4",
		Tokens: []Token{
			{Kind: TokenSpecial, Name: "sp", Min: 1, Max: 3, Offset: 4},
		},
	}, got.Patterns[0])

	assert.Equal(t, Pattern{
		Path: "Show Part {se} - {ep} - *",
		Tokens: []Token{
			{Kind: TokenSeason, Name: "se"},
			{Kind: TokenEpisode, Name: "ep"},
		},
	}, got.Patterns[2])
}

func TestParsePatternTokens(t *testing.T) {
	for _, tc := range []struct {
		path string
		want []Token
	}{
		{"{s}{e}", []Token{{Kind: TokenSeason, Name: "s"}, {Kind: TokenEpisode, Name: "e"}}},
		{"{season}-{ep}", []Token{{Kind: TokenSeason, Name: "season"}, {Kind: TokenEpisode, Name: "ep"}}},
		{"{e}", []Token{{Kind: TokenEpisode, Name: "e"}}},
		{"{episode}", []Token{{Kind: TokenEpisode, Name: "episode"}}},
		{"{special}", []Token{{Kind: TokenSpecial, Name: "special"}}},
		{"{see}", []Token{{Kind: TokenSeasonEpisode, Name: "see"}}},
		{"{ep,l2}", []Token{{Kind: TokenEpisode, Name: "ep", MaxLength: 2}}},
		{"{ep,-3}", []Token{{Kind: TokenEpisode, Name: "ep", Offset: -3}}},
		{"{ E , 1 - 3 }", []Token{{Kind: TokenEpisode, Name: "E", Min: 1, Max: 3}}},
		{"{ep}{ep}", []Token{{Kind: TokenEpisode, Name: "ep"}, {Kind: TokenEpisode, Name: "ep"}}},
		{"{ep} / {se}", []Token{{Kind: TokenEpisode, Name: "ep"}, {Kind: TokenSeason, Name: "se"}}},
		{"literal {ep} and \\{not a token\\}", []Token{{Kind: TokenEpisode, Name: "ep"}}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			got, err := ParsePattern(tc.path)
			require.NoError(t, err)
			assert.Equal(t, tc.path, got.Path)
			assert.Equal(t, tc.want, got.Tokens)
		})
	}
}

func TestParsePatternErrors(t *testing.T) {
	for _, path := range []string{
		"",
		"   ",
		"Bonus/*.mp4",              // no match token captures an episode
		"{season}-{s}",             // still no episode number
		"Bonus/Bonus {sp,1-3.mp4",  // unterminated
		"Bonus/Bonus {nope}.mp4",   // unknown token
		"Bonus/Bonus {sp,3-1}.mp4", // descending range
		"Bonus/Bonus {sp,}.mp4",    // empty parameter
		"Bonus/Bonus {sp,x}.mp4",   // bad parameter
		"Bonus/Bonus {ep,l0}.mp4",  // bad length
		"Bonus/{ep{se}}.mp4",       // nested token
	} {
		t.Run(path, func(t *testing.T) {
			_, err := ParsePattern(path)
			assert.Error(t, err)
		})
	}
}

func TestUnmarshalSyntaxErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
		line int
	}{
		{"missing colon", "title: Cool Show\nbroken line\n", 2},
		{"missing hint name", "  : value\n", 1},
		{"empty title", "title:\n", 1},
		{"bad year", "year: soon\n", 1},
		{"negative season", "season: -1\n", 1},
		{"bad imdb id", "imdbid: tt\n", 1},
		{"empty guid", "guid:  \n", 1},
		{"episode without file", "ep: 01:\n", 1},
		{"episode without colon", "ep: 01 file.mkv\n", 1},
		{"space before the episode colon", "ep: 01 : file.mkv\n", 1},
		{"bad episode number", "ep: 01x02: file.mkv\n", 1},
		{"bad episode range", "ep: 02-01: file.mkv\n", 1},
		{"pattern without episode token", "pattern: Bonus/*.mp4\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Unmarshal([]byte(tc.data))
			var syntax *SyntaxError
			require.ErrorAs(t, err, &syntax)
			assert.Equal(t, tc.line, syntax.Line)
		})
	}
}

func TestUnmarshalKeepsHintsBeforeAnError(t *testing.T) {
	got, err := Unmarshal([]byte("title: Cool Show\nnonsense\n"))
	require.Error(t, err)
	assert.Equal(t, "Cool Show", got.Title)
}

func TestUnmarshalWhitespaceAndLineEndings(t *testing.T) {
	// A byte order mark, CRLF line endings and a value in which the spaces
	// belong to the file name.
	got, err := Unmarshal([]byte("\xef\xbb\xbfTitle:   Cool Show  \r\n\r\n   \t\r\nep: 01:  A Name.mkv \r\n"))
	require.NoError(t, err)
	assert.Equal(t, "Cool Show", got.Title)
	assert.Equal(t, []Episode{{Name: "A Name.mkv", Episode: 1}}, got.Episodes)
}

func TestUnmarshalEmpty(t *testing.T) {
	got, err := Unmarshal(nil)
	require.NoError(t, err)
	assert.Equal(t, PlexMatch{}, got)
}

func TestMarshal(t *testing.T) {
	got := Marshal(PlexMatch{
		Title:  "Cool Show",
		Year:   2018,
		Season: 1,
		TMDBID: 203737,
		GUID:   "plex://show/5d9c07fd02391c001f57e02f",
		Episodes: []Episode{
			{Name: "Oshi no Ko - 02.mkv", Episode: 2},
			{Name: "Oshi no Ko - 01.mkv", Episode: 1},
			{Name: "Oshi no Ko - sp02.mkv", Special: true, Episode: 2},
			{Name: "Oshi no Ko - sp01.mkv", Special: true, Episode: 1},
			// The same episode as the first hint; Unique drops it.
			{Name: "Oshi no Ko - 02 again.mkv", Episode: 2},
		},
	})
	want := "title: Cool Show\n" +
		"\n" +
		"year: 2018\n" +
		"\n" +
		"season: 1\n" +
		"\n" +
		"tmdbid: 203737\n" +
		"\n" +
		"guid: plex://show/5d9c07fd02391c001f57e02f\n" +
		"\n" +
		"ep: SP01: Oshi no Ko - sp01.mkv\n" +
		"ep: SP02: Oshi no Ko - sp02.mkv\n" +
		"ep: 01: Oshi no Ko - 01.mkv\n" +
		"ep: 02: Oshi no Ko - 02.mkv\n" +
		"ep: 02: Oshi no Ko - 02 again.mkv\n"
	assert.Equal(t, want, string(got))
}

func TestMarshalDoesNotTouchTheCallerSlice(t *testing.T) {
	episodes := []Episode{{Name: "b.mkv", Episode: 2}, {Name: "a.mkv", Episode: 1}}
	Marshal(PlexMatch{Episodes: episodes})
	assert.Equal(t, []Episode{{Name: "b.mkv", Episode: 2}, {Name: "a.mkv", Episode: 1}}, episodes)
}

func TestMarshalUnknownHints(t *testing.T) {
	got := Marshal(PlexMatch{
		Title:        "Cool Show",
		UnknownHints: []UnknownHint{{Name: "FutureHint", Value: "something"}},
	})
	assert.Equal(t, "title: Cool Show\n\nFutureHint: something\n", string(got))
}

func TestRoundTrip(t *testing.T) {
	data := `# A file that uses every hint this package knows.
Title: Cool Show
Year: 2018
Season: 2
TvdbId: 377269
TmdbId: 203737
ImdbId: tt11823076
Guid: plex://show/5d9c07fd02391c001f57e02f

pattern: Bonus/Bonus {sp,1-3,+4}.mp4

ep: SP01: Some Unnumbered Special.mkv
ep: S03E12-S03E13: A Double Episode.mkv
ep: 01: Pilot.mkv
somethingnew: kept
`
	first, err := Unmarshal([]byte(data))
	require.NoError(t, err)

	out := Marshal(first)
	second, err := Unmarshal(out)
	require.NoError(t, err, "marshalled file:\n%s", out)

	assert.Equal(t, out, Marshal(second), "marshal is stable")
	// Marshal writes the episodes in its own order; everything else has to
	// survive the round trip unchanged.
	assert.Equal(t, uniqueEpisodes(first.Episodes), second.Episodes)
	first.Episodes, second.Episodes = nil, nil
	assert.Equal(t, first, second)
}

func TestMerge(t *testing.T) {
	series := PlexMatch{
		Title:        "Cool Show",
		Year:         2018,
		TMDBID:       203737,
		Episodes:     []Episode{{Name: "Season 1/Pilot.mkv", Episode: 1}},
		UnknownHints: []UnknownHint{{Name: "one", Value: "1"}},
	}
	season := PlexMatch{
		Season:   2,
		GUID:     "plex://show/5d9c07fd02391c001f57e02f",
		Episodes: []Episode{{Name: "Winter Comes.mkv", Episode: 1}},
	}
	assert.Equal(t, PlexMatch{
		Title:    "Cool Show",
		Year:     2018,
		Season:   2,
		TMDBID:   203737,
		GUID:     "plex://show/5d9c07fd02391c001f57e02f",
		Episodes: []Episode{{Name: "Season 1/Pilot.mkv", Episode: 1}, {Name: "Winter Comes.mkv", Episode: 1}},
		UnknownHints: []UnknownHint{
			{Name: "one", Value: "1"},
		},
	}, series.Merge(season))
}

func TestValidate(t *testing.T) {
	require.NoError(t, PlexMatch{Title: "Cool Show"}.Validate())
	require.NoError(t, PlexMatch{Episodes: []Episode{
		{Name: "a.mkv", Season: 3, Episode: 12, LastEpisode: 13},
		{Name: "b.mkv", Special: true, Episode: 1},
	}}.Validate())

	for _, tc := range []struct {
		name string
		p    PlexMatch
	}{
		{"negative id", PlexMatch{TMDBID: -1}},
		{"episode without a file name", PlexMatch{Episodes: []Episode{{Episode: 1}}}},
		{"episode zero", PlexMatch{Episodes: []Episode{{Name: "a.mkv"}}}},
		{"special in a season", PlexMatch{Episodes: []Episode{{Name: "a.mkv", Special: true, Season: 1, Episode: 1}}}},
		{"descending range", PlexMatch{Episodes: []Episode{{Name: "a.mkv", Episode: 2, LastEpisode: 1}}}},
		{"last season without a last episode", PlexMatch{Episodes: []Episode{{Name: "a.mkv", Episode: 1, LastSeason: 2}}}},
		{"bad pattern", PlexMatch{Patterns: []Pattern{{Path: "Bonus/*.mp4"}}}},
		{"empty pattern", PlexMatch{Patterns: []Pattern{{Path: ""}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Error(t, tc.p.Validate())
		})
	}
}

// TestDocumentationExamples covers the examples in the Plex documentation at
// https://support.plex.tv/articles/plexmatch/.
func TestDocumentationExamples(t *testing.T) {
	got, err := Unmarshal([]byte(`# This directory contains the second season of Cool Show
Title: Cool Show

# The series' first season originally aired in 2018
Year: 2018
Season: 2

# We could also add a tvdbid: hint if we knew the ID for
# Cool Show, but we'd still need to provide the season
# hint as well.
`))
	require.NoError(t, err)
	assert.Equal(t, PlexMatch{Title: "Cool Show", Year: 2018, Season: 2}, got)

	got, err = Unmarshal([]byte(`# Bonus clips 1 through 3 are numbered 5 through 8 on TVDB
Pattern: Bonus/Bonus {sp,1-3,+4}.mp4
# Clips 4 through 6 are numbered 10 through 12
Pattern: Bonus/Bonus {sp,4-6,+6}.mp4

# These files use an unusual naming convention
Pattern: Show Part {se} - {ep} - *

# Regular episodes with tricky filenames
Episode: 01: Pilot.mkv
ep: 12: Episode 12 - Finale Part 2.mkv

# Specials
ep: SP01: Some Unnumbered Special.mkv
ep: SP03: Some Special Subseries 1.mkv
ep: SP04: Some Special Subseries 2.mkv

ep: SP02: Season 1/Bonus/Bonus Clip 1.mkv
ep: SP04: Season 2/Bonus/Bonus Clip 1.mkv
`))
	require.NoError(t, err)
	require.Len(t, got.Patterns, 3)
	require.Len(t, got.Episodes, 7)
	assert.Equal(t, Episode{Name: "Season 1/Bonus/Bonus Clip 1.mkv", Special: true, Episode: 2}, got.Episodes[5])
}

// TestSonarrStyleFile covers the shape of the .plexmatch files that Sonarr's
// Plex metadata agent writes.
func TestSonarrStyleFile(t *testing.T) {
	got, err := Unmarshal([]byte(`Title: Tiger King
Year: 2020
TvdbId: 377269
ImdbId: tt11823076
Episode: S01E01: Season 01/Tiger King - S01E01 - Not Your Average Joe.mkv
Episode: S01E02: Season 01/Tiger King - S01E02 - Cult of Personality.mkv
Episode: S02E01: Season 02/Tiger King - S02E01 - Beg Your Pardon HDTV-1080p.mkv
`))
	require.NoError(t, err)
	assert.Equal(t, "Tiger King", got.Title)
	assert.Equal(t, 2020, got.Year)
	assert.Equal(t, 377269, got.TVDBID)
	assert.Equal(t, 11823076, got.IMDBID)
	require.Len(t, got.Episodes, 3)
	assert.Equal(t, Episode{
		Name:    "Season 01/Tiger King - S01E01 - Not Your Average Joe.mkv",
		Season:  1,
		Episode: 1,
	}, got.Episodes[0])
	assert.NoError(t, got.Validate())
}

func TestValidateRejectsValuesThatBreakALine(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    PlexMatch
	}{
		{"title", PlexMatch{Title: "Cool\nShow"}},
		{"guid", PlexMatch{GUID: "plex://show/a\r\nb"}},
		{"file name", PlexMatch{Episodes: []Episode{{Name: "a\nb.mkv", Episode: 1}}}},
		{"pattern", PlexMatch{Patterns: []Pattern{{Path: "Bonus/{ep}\n{sp}"}}}},
		{"unknown hint name", PlexMatch{UnknownHints: []UnknownHint{{Name: "two:names", Value: "x"}}}},
		{"empty hint name", PlexMatch{UnknownHints: []UnknownHint{{Name: "", Value: "x"}}}},
		{"unknown hint value", PlexMatch{UnknownHints: []UnknownHint{{Name: "a", Value: "x\ny"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Error(t, tc.p.Validate())
		})
	}
}

// FuzzUnmarshal checks that reading never fails on a panic, that whatever is
// read can be written back, and that what is written back reads the same.
func FuzzUnmarshal(f *testing.F) {
	for _, seed := range []string{
		"",
		"\n\n# only a comment\n",
		"title: Cool Show\nshow: Other\nyear: 2018\nseason: 2\n",
		"tvdbid: 377269\ntmdbid: 203737\nimdbid: tt11823076\nguid: plex://show/abc\n",
		"ep: 01: a.mkv\nep: SP01: b.mkv\nep: S01E03-E04: c.mkv\nep: 12-13: d.mkv\n",
		"pattern: Bonus/Bonus {sp,1-3,+4}.mp4\n",
		"pattern: {see}/{ep,l2}\n",
		"ep: 01 : a.mkv\n",
		"nonsense\ntitle: Cool Show\n",
		"\xef\xbb\xbftitle: Cool Show\r\n\r\nep: 1: a.mkv\r\n",
		"title: \n",
		"somehint: 0\r0\n",
		"ep: 01: a.mkv\rsomehint: 0\r0\n",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := Unmarshal(data)
		if err != nil {
			// Reading stopped part way; what it did read must still be
			// usable.
			require.NoError(t, p.Validate())
			return
		}
		require.NoError(t, p.Validate(), "a file that was read must validate")

		out := Marshal(p)
		again, err := Unmarshal(out)
		require.NoError(t, err, "marshalled file:\n%s", out)
		require.Equal(t, out, Marshal(again), "marshal is stable")
	})
}

func TestUnique(t *testing.T) {
	p := PlexMatch{Episodes: []Episode{
		{Name: "b.mkv", Episode: 2},
		{Name: "a.mkv", Episode: 1},
		{Name: "also two.mkv", Episode: 2},
		{Name: "sp.mkv", Special: true, Episode: 1},
	}}
	assert.Equal(t, []Episode{
		{Name: "sp.mkv", Special: true, Episode: 1},
		{Name: "a.mkv", Episode: 1},
		{Name: "b.mkv", Episode: 2},
	}, p.Unique().Episodes)

	// A hint that names a range and one that names a single episode of it
	// are different claims, and both are kept.
	p = PlexMatch{Episodes: []Episode{
		{Name: "both.mkv", Episode: 1, LastEpisode: 2},
		{Name: "one.mkv", Episode: 1},
	}}
	assert.Len(t, p.Unique().Episodes, 2)

	// Unique copies rather than edits.
	p = PlexMatch{Episodes: []Episode{{Name: "b.mkv", Episode: 2}, {Name: "a.mkv", Episode: 1}}}
	p.Unique()
	assert.Equal(t, []Episode{{Name: "b.mkv", Episode: 2}, {Name: "a.mkv", Episode: 1}}, p.Episodes)

	assert.Equal(t, PlexMatch{Title: "Cool Show"}, PlexMatch{Title: "Cool Show"}.Unique())
}

func TestEpisodeResolveSeason(t *testing.T) {
	assert.Equal(t, 3, Episode{Name: "a.mkv", Episode: 1}.ResolveSeason(3), "no season named")
	assert.Equal(t, 5, Episode{Name: "a.mkv", Season: 5, Episode: 1}.ResolveSeason(3), "season named")
	assert.Equal(t, 0, Episode{Name: "a.mkv", Special: true, Episode: 1}.ResolveSeason(3), "special")
	assert.Equal(t, 1, Episode{Name: "a.mkv", Episode: 1}.ResolveSeason(1), "season 1 directory")
}

func TestTokenHasRange(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"{sp,1-3}.mp4", true},
		{"{sp,0-3}.mp4", true},
		{"{sp,+4}.mp4", false},
		{"{ep}.mp4", false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			got, err := ParsePattern(tc.path)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.Tokens[0].HasRange())
		})
	}
}
