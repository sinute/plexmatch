[中文](README_zh.md)

# Introduction

plexmatch provides structured parsing and generation of [.plexmatch](https://support.plex.tv/articles/plexmatch/) files, the match-hint files Plex Media Server uses to match a TV series without renaming it.

It reads the whole format:

* series and season hints — `title`/`show`, `year`, `season`, `tmdbid`, `tvdbid`, `imdbid` and `guid`
* episode hints — `ep`/`episode`, in every number form Plex accepts
* pattern hints — `pattern`/`pt`, with their match tokens
* `#` comments, blank lines, case-insensitive hint names and CRLF line endings

The module path is `github.com/sinute/plexmatch/v2`, and upgrading from v1 is a few renames; see [Upgrading from v1](#upgrading-from-v1).

```
go get github.com/sinute/plexmatch/v2
```

# Usage

## Parse a .plexmatch file

```go
package main

import (
	"fmt"

	"github.com/sinute/plexmatch/v2"
)

func main() {
	data := `# This directory holds the first season of Cool Show.
Title: Cool Show
Year: 2018
Season: 1
TvdbId: 377269
ImdbId: tt11823076

ep: SP01: Oshi no Ko - sp.mkv
ep: 01: Oshi no Ko - 01.mkv
ep: S01E03-E04: Oshi no Ko - 03-04.mkv
pattern: Bonus/Bonus {sp,1-3,+4}.mp4
`
	p, err := plexmatch.Unmarshal([]byte(data))
	if err != nil {
		panic(err)
	}
	fmt.Println(p.Title, p.Year, p.Season, p.TVDBID, p.IMDBID)
	// Cool Show 2018 1 377269 11823076

	for _, ep := range p.Episodes {
		fmt.Println(ep.Number(), ep.Name, ep.ResolveSeason(p.Season), ep.LastEpisode)
	}
	// SP01 Oshi no Ko - sp.mkv 0 0
	// 01 Oshi no Ko - 01.mkv 1 0
	// S01E03-S01E04 Oshi no Ko - 03-04.mkv 1 4

	for _, token := range p.Patterns[0].Tokens {
		fmt.Println(token.Kind, token.Min, token.Max, token.Offset)
	}
	// special 1 3 4
}
```

A malformed line stops the parse and is reported as a `*plexmatch.SyntaxError` that names the line:

```go
_, err := plexmatch.Unmarshal([]byte("title: Cool Show\nep: 01 file.mkv\n"))
// plexmatch: line 2: missing ':' between episode number and file name: "ep: 01 file.mkv"
```

## Generate a .plexmatch file

```go
package main

import (
	"fmt"

	"github.com/sinute/plexmatch/v2"
)

func main() {
	p := plexmatch.PlexMatch{
		Title:  "Cool Show",
		Year:   2018,
		Season: 2,
		Episodes: []plexmatch.Episode{
			{Name: "A Special.mkv", Special: true, Episode: 1},
			{Name: "An Ordinary Name.mkv", Episode: 2},
			{Name: "A Double Episode.mkv", Season: 2, Episode: 3, LastEpisode: 4},
		},
	}
	p = p.Unique()
	if err := p.Validate(); err != nil {
		panic(err)
	}
	fmt.Print(string(plexmatch.Marshal(p)))
	// title: Cool Show
	//
	// year: 2018
	//
	// season: 2
	//
	// ep: SP01: A Special.mkv
	// ep: 02: An Ordinary Name.mkv
	// ep: S02E03-S02E04: A Double Episode.mkv
}
```

`Marshal` writes every hint it is given, sorting the episodes by season and episode number with specials first. `Unique` returns a copy with the episodes sorted and any hint that claims an episode an earlier hint already claims dropped, and `Validate` checks a `PlexMatch` that was built in Go against the constraints Plex places on hint values. `Merge` lays the `.plexmatch` file of a season directory over the one of its series directory.

# The episode number

An `ep` hint maps one file to the episodes it contains. The number before the colon may be written in every form Plex accepts, and it is held in the `Episode` struct as shown:

| Written         | Season | Special | Episode | LastEpisode | LastSeason |
| --------------- | -----: | ------: | ------: | ----------: | ---------: |
| `12`            |      0 |   false |      12 |           0 |          0 |
| `E12`           |      0 |   false |      12 |           0 |          0 |
| `SP04`          |      0 |    true |       4 |           0 |          0 |
| `S03E04`        |      3 |   false |       4 |           0 |          0 |
| `01-02`         |      0 |   false |       1 |           2 |          0 |
| `S03E12-S03E13` |      3 |   false |      12 |          13 |          0 |
| `S03E12-S04E01` |      3 |   false |      12 |           1 |          4 |

A season of 0 with `Special` false means the hint names no season of its own: the season comes from the directory, from a `season` hint, or from the caller. `ResolveSeason` answers that for a `PlexMatch` that was read from a file:

```go
for _, ep := range p.Episodes {
	fmt.Println(ep.Name, ep.ResolveSeason(p.Season))
}
```

`S00Exx` is read as a special and written back as `SPxx`.

# Upgrading from v1

v2 reads the parts of the format v1 could not, which moved the API. It is a separate module, so v1 keeps working for code that has not moved yet.

## 1. Change the import path

```
go get github.com/sinute/plexmatch/v2
```

```diff
-import "github.com/sinute/plexmatch"
+import "github.com/sinute/plexmatch/v2"
```

## 2. Rename two fields

| v1                     | v2                     | Why                                                                                     |
| ---------------------- | ---------------------- | --------------------------------------------------------------------------------------- |
| `PlexMatch.Show`       | `PlexMatch.Title`      | `title` and `show` are synonyms in the format, so both hints set `Title` and a file can no longer disagree with itself. |
| `Episode.ID`           | `Episode.Episode`      | The episode number, now beside `Season`, `LastEpisode` and `LastSeason`.                 |
| `Episode.Name`, `Episode.Special` | unchanged   |                                                                                          |

`Unmarshal` and `Marshal` keep their signatures.

```diff
 p := plexmatch.PlexMatch{
 	Title:  "【推しの子】",
 	TMDBID: 203737,
 	Season: 1,
 	Episodes: []plexmatch.Episode{
-		{ID: 1, Name: "Oshi no Ko - sp.mkv", Special: true},
-		{ID: 1, Name: "Oshi no Ko - 01.mkv", Special: false},
+		{Name: "Oshi no Ko - sp.mkv", Special: true, Episode: 1},
+		{Name: "Oshi no Ko - 01.mkv", Episode: 1},
 	},
 }
```

If you set `Show`, set `Title` instead. If you read `Show` after `Unmarshal`, read `Title`: a `show:` hint now fills `Title`.

## 3. Check for the changes the compiler will not catch

* **`Marshal` no longer drops hints.** v1 silently wrote one hint per episode, keeping whichever file name came first. `Marshal` now writes every hint it is given, so nothing is lost between reading a file and writing it back. If you relied on the old behaviour, call `Unique` first:

  ```diff
  -out := plexmatch.Marshal(p)
  +out := plexmatch.Marshal(p.Unique())
  ```

* **`Marshal` no longer sorts your slice in place.** It still writes the episodes sorted by season and episode number with specials first, but it sorts a copy, so a `PlexMatch` you pass in is unchanged.

* **`Unmarshal` fails on lines v1 misread.** v1 ignored a hint it did not recognise and rejected some it should have accepted. Now a line that is not a hint at all — no colon, an empty value, a bad episode number — is an error, reported as a `*plexmatch.SyntaxError` carrying the line number, the offending text and a message. The hints read before it are still returned:

  ```go
  p, err := plexmatch.Unmarshal(data)
  var syntax *plexmatch.SyntaxError
  if errors.As(err, &syntax) {
      fmt.Println(syntax.Line, syntax.Msg)
  }
  ```

* **An unknown hint is kept, not rejected.** A directive this package does not know ends up in `PlexMatch.UnknownHints` as an `UnknownHint` and is written back out, so a file that a newer Plex Media Server understands survives a round trip through this package.

## 4. Take up what is new

None of it is required, but it is why v2 exists:

* `PlexMatch.GUID` — the `guid` hint, which v1's grammar had a token for but never read.
* `PlexMatch.Patterns` — `pattern`/`pt` hints and their match tokens, which v1 did not support at all.
* Episode hints now accept the `E02` and `S03E04` forms, multi-episode ranges such as `01-02` and `S01E01-E02`, and the `tt` prefix on `imdbid`.
* Comments, blank lines, CRLF line endings and a byte order mark are handled, and hint names are matched case-insensitively.
* `Validate`, `Unique` and `Merge`.
* `Episode.Number`, `Episode.String` and `Pattern.String` render a hint the way a `.plexmatch` file writes it.

## A note on the parser

The hand-written line parser replaced the goyacc grammar. The format is line oriented, so the grammar earned little; `plexmatch.y`, the generated parser and lexer, `y.output` and the `goyacc` step of the Makefile are gone, and the parse errors now name a line instead of a parser state.
