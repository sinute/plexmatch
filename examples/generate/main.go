// Command generate builds a .plexmatch file.
package main

import (
	"fmt"

	"github.com/sinute/plexmatch"
)

func main() {
	p := plexmatch.PlexMatch{
		Title:  "Cool Show",
		Year:   2018,
		Season: 2,
		GUID:   "plex://show/5d9c07fd02391c001f57e02f",
		Episodes: []plexmatch.Episode{
			{Name: "An Ordinary Name.mkv", Episode: 2},
			{Name: "A Special.mkv", Special: true, Episode: 1},
			{Name: "A Double Episode.mkv", Season: 2, Episode: 3, LastEpisode: 4},
			{Name: "Tricky Part 3.mkv", Episode: 1, Season: 2},
		},
		Patterns: []plexmatch.Pattern{
			{Path: "Bonus/Bonus {sp,1-3,+4}.mp4"},
		},
	}
	if err := p.Validate(); err != nil {
		panic(err)
	}
	fmt.Print(string(plexmatch.Marshal(p)))
}
