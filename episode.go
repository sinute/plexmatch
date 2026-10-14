package plexmatch

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Episode is an "ep" or "episode" hint: it maps one file to the episode, or
// range of episodes, it contains.
//
// An episode number may name its season ("S03E04") or not ("04", "E04"), in
// which case the season of the surrounding directory, or a "season" hint,
// applies. Specials are season 0 and are written with the "SP" form ("SP04"),
// which is what the Special field records.
type Episode struct {
	// Name is the file the hint applies to: a file name, or a path relative
	// to the directory that holds the .plexmatch file, using forward slashes
	// as separators ("Season 1/Bonus/Clip 1.mkv").
	Name string

	// Season is the season the file belongs to. It is 0 for specials and for
	// hints that do not name a season.
	Season int

	// Special reports the "SP" form of a season 0 episode, as in "SP04".
	// Setting it implies Season 0.
	Special bool

	// Episode is the first episode number the file contains.
	Episode int

	// LastEpisode is the last episode number of a file that holds several
	// episodes ("01-02", "S01E01-E02", "S03E12-S03E13"). It is 0 when the
	// file holds a single episode.
	LastEpisode int

	// LastSeason is the season of LastEpisode when the hint names one that
	// differs from Season ("S03E12-S04E01"). It is 0 when the range ends in
	// the same season it starts in, which is the usual case.
	LastSeason int
}

// ResolveSeason returns the season the episode belongs to, given the season of
// the directory that holds the .plexmatch file, or the season of a "season"
// hint in it.
//
// A special is season 0 and a hint that names its own season answers for
// itself, so directorySeason is used only for a hint that names no season.
func (e Episode) ResolveSeason(directorySeason int) int {
	switch {
	case e.Special:
		return 0
	case e.Season != 0:
		return e.Season
	default:
		return directorySeason
	}
}

// errEpisodeNumber reports an episode number this package cannot read.
var errEpisodeNumber = errors.New("invalid episode number")

// epNumber is an episode number as written, before it becomes an Episode.
type epNumber struct {
	season      int
	episode     int
	seasonGiven bool
	special     bool
}

// parseEpisode parses the value of an "ep" hint: an episode number, a colon,
// and the file the hint applies to.
func parseEpisode(value string) (Episode, error) {
	number, name, ok := strings.Cut(value, ":")
	if !ok {
		return Episode{}, errors.New("missing ':' between episode number and file name")
	}
	name = trimSpace(name)
	if name == "" {
		return Episode{}, errors.New("missing file name")
	}
	if number == "" || strings.ContainsAny(number, " \t\v\f\r") {
		return Episode{}, fmt.Errorf("invalid episode number %q", number)
	}
	e := Episode{Name: name}
	if err := e.setNumber(number); err != nil {
		return Episode{}, err
	}
	if err := e.validate(); err != nil {
		return Episode{}, err
	}
	return e, nil
}

// setNumber reads an episode number such as "04", "SP04", "S03E04" or
// "S01E01-E02".
func (e *Episode) setNumber(number string) error {
	s := asciiLower(number)
	first, n, err := scanEpisodeNumber(s)
	if err != nil {
		return err
	}
	e.Season, e.Special, e.Episode = first.season, first.special, first.episode

	rest := s[n:]
	if rest == "" {
		return nil
	}
	if rest[0] != '-' {
		return fmt.Errorf("%w %q", errEpisodeNumber, number)
	}
	last, n, err := scanEpisodeNumber(rest[1:])
	if err != nil || n != len(rest)-1 {
		return fmt.Errorf("%w %q", errEpisodeNumber, number)
	}
	e.LastEpisode = last.episode
	if last.seasonGiven && last.season != e.Season {
		e.LastSeason = last.season
	}
	return nil
}

// scanEpisodeNumber reads the episode number at the start of s, which must be
// lower case, and reports how many bytes it consumed.
func scanEpisodeNumber(s string) (epNumber, int, error) {
	switch {
	case strings.HasPrefix(s, "sp"):
		n, size := scanDigits(s[2:])
		if size == 0 {
			return epNumber{}, 0, errEpisodeNumber
		}
		return epNumber{special: true, seasonGiven: true, episode: n}, 2 + size, nil
	case strings.HasPrefix(s, "s"):
		season, size := scanDigits(s[1:])
		if size == 0 {
			return epNumber{}, 0, errEpisodeNumber
		}
		rest := s[1+size:]
		if !strings.HasPrefix(rest, "e") {
			return epNumber{}, 0, errEpisodeNumber
		}
		episode, size2 := scanDigits(rest[1:])
		if size2 == 0 {
			return epNumber{}, 0, errEpisodeNumber
		}
		// Season 0 is the season of specials, whichever spelling was used.
		return epNumber{season: season, special: season == 0, seasonGiven: true, episode: episode}, 1 + size + 1 + size2, nil
	case strings.HasPrefix(s, "e"):
		n, size := scanDigits(s[1:])
		if size == 0 {
			return epNumber{}, 0, errEpisodeNumber
		}
		return epNumber{episode: n}, 1 + size, nil
	}
	n, size := scanDigits(s)
	if size == 0 {
		return epNumber{}, 0, errEpisodeNumber
	}
	return epNumber{episode: n}, size, nil
}

// scanDigits reads the leading decimal digits of s and reports their value and
// how many bytes they occupy.
func scanDigits(s string) (int, int) {
	i := 0
	for i < len(s) && '0' <= s[i] && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, 0
	}
	n, err := strconv.Atoi(s[:i])
	if err != nil {
		return 0, 0
	}
	return n, i
}

// Number returns the episode number of e in the form a .plexmatch file uses,
// for example "04", "SP04", "S03E04" or "S01E01-E02".
func (e Episode) Number() string {
	first := episodeNumberString(e.Season, e.Special, e.Episode)
	if e.LastEpisode == 0 {
		return first
	}
	season, special := e.Season, e.Special
	if e.LastSeason != 0 {
		season, special = e.LastSeason, false
	}
	return first + "-" + episodeNumberString(season, special, e.LastEpisode)
}

// String returns the hint value of e, an episode number and a file name, as
// it is written after "ep: ".
func (e Episode) String() string {
	return e.Number() + ": " + e.Name
}

func episodeNumberString(season int, special bool, number int) string {
	switch {
	case special:
		return fmt.Sprintf("SP%02d", number)
	case season != 0:
		return fmt.Sprintf("S%02dE%02d", season, number)
	default:
		return fmt.Sprintf("%02d", number)
	}
}

func (e Episode) validate() error {
	if trimSpace(e.Name) == "" {
		return errors.New("empty file name")
	}
	if err := checkLine("file name", e.Name); err != nil {
		return err
	}
	if e.Episode <= 0 {
		return fmt.Errorf("invalid episode number %d", e.Episode)
	}
	if e.Season < 0 || e.LastSeason < 0 || e.LastEpisode < 0 {
		return errors.New("negative season or episode number")
	}
	if e.Special && e.Season != 0 {
		return fmt.Errorf("special episode in season %d, want season 0", e.Season)
	}
	switch {
	case e.LastEpisode == 0 && e.LastSeason != 0:
		return errors.New("last season without a last episode")
	case e.LastEpisode != 0:
		season := e.LastSeason
		if season == 0 {
			season = e.Season
		}
		if season < e.Season || (season == e.Season && e.LastEpisode <= e.Episode) {
			return fmt.Errorf("episode range %d-%d is not ascending", e.Episode, e.LastEpisode)
		}
	}
	return nil
}
