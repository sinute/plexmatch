// Command parse reads a .plexmatch file and prints the hints it holds.
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
Guid: plex://show/5d9c07fd02391c001f57e02f

# Bonus clips are numbered 5 through 7 on TVDB.
pattern: Bonus/Bonus {sp,1-3,+4}.mp4

ep: SP01: A Tricky File Name.mkv
ep: 02: An Ordinary Name.mkv
ep: S01E03-E04: A Double Episode.mkv
`
	p, err := plexmatch.Unmarshal([]byte(data))
	if err != nil {
		panic(err)
	}
	fmt.Printf("title:  %s\n", p.Title)
	fmt.Printf("year:   %d\n", p.Year)
	fmt.Printf("season: %d\n", p.Season)
	fmt.Printf("tvdb:   %d\n", p.TVDBID)
	fmt.Printf("imdb:   %d\n", p.IMDBID)
	fmt.Printf("guid:   %s\n", p.GUID)
	for _, e := range p.Episodes {
		fmt.Printf("ep:     %s\n", e)
	}
	for _, pattern := range p.Patterns {
		fmt.Printf("pattern: %s\n", pattern.Path)
		for _, token := range pattern.Tokens {
			fmt.Printf("  token %s (%s) range %d-%d offset %d\n",
				token.Name, token.Kind, token.Min, token.Max, token.Offset)
		}
	}
}
