/**
 * Copyright 2021 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package changelog

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

const (
	headerBlock   int = 0
	titleBlock    int = iota
	releasesBlock int = iota
	linksBlock    int = iota
)

var (
	emptyLine      = regexp.MustCompile(`^\s*$`)
	commentStart   = regexp.MustCompile(`^\s*<!--.*$`)
	commentEnd     = regexp.MustCompile(`-->\s*$`)
	titleRE        = regexp.MustCompile(`^\s*#\s*(.*)\s*$`)
	semverRE       = regexp.MustCompile(`https?://semver.org/spec/(.*?).html`)
	changelogVerRE = regexp.MustCompile(`https?://keepachangelog.com/[^/]*/([^/]*)/`)
	releaseRE      = regexp.MustCompile(`^\s*##\s*(\[([^\]]*)\](\s*-\s*(\d{4}-\d\d-\d\d))?\s*(\[\s*YANKED\s*\])?)\s*$`)
	detailsRE      = regexp.MustCompile(`^\s*###\s*(Added|Changed|Deprecated|Fixed|Removed|Security)\s*$`)
	linksRE        = regexp.MustCompile(`^\s*\[([^\]]*)\]\s*:\s*(https?://.*?)\s*$`)
)

// Release represents a documented release.
type Release struct {
	// The entire text following the ## prefix.
	Title string

	// The version of the release.  It could be v1.0.2, 1.0.3-pre1 or Unreleased
	// as examples.
	Version string

	// The date of the release if present.
	Date *time.Time

	// If a release has been yanked.
	Yanked bool

	// Detailed types of changes follow.  If there are multiple instances of
	// the header, they are merged together and stored in the order they appear
	// in the file.

	// The lines under the '### Added` header.
	Added []string

	// The lines under the '### Changed' header.
	Changed []string

	// The lines under the '### Depreceted' header.
	Deprecated []string

	// The lines under the '### Removed' header.
	Removed []string

	// The lines under the '### Fixed' header.
	Fixed []string

	// The lines under the '### Security' header.
	Security []string

	// The lines the might immediately follow the release line but not be
	// associated with any specific header.
	Other []string

	// The entire body of the release in case that is useful.
	Body []string
}

// The Link structure containing the release version and the representing URL.
type Link struct {
	// The version of the release.  It could be v1.0.2, 1.0.3-pre1 or Unreleased
	// as examples.
	Version string

	// The following URL that describes the difference between this release and
	// the previous release
	Url string
}

// The Changelog structure contains the entire changelog.  It may be populated
// from a file or programatically.
type Changelog struct {
	// The https://keepachangelog.com version of this file.
	KeepAChangelogVersion string

	// The semantic version policy this file conforms.
	SemVerVersion string

	// Any header comments that might exist before the main body of contents.
	CommentHeader []string

	// The title of the changelog.  Generally will be "Changelog".
	Title string

	// The description body about the change log and what it conforms to.
	Description []string

	// The collection of releases provided by this changelog file.
	Releases []Release

	// The collection of links showing the differences between release versions.
	Links []Link
}

// The Opts used for parsing the input stream.
type Opts struct {
	AllowInconsistentCase bool // TODO: Ignore the case of keywords in the markdown
	EnforceDateIsPresent  bool // TODO: Enforce that a date is present and correct
}

// Parse takes a bufio.Scanner and processes the file into
func Parse(r io.Reader, opts *Opts) (*Changelog, error) {
	var lastRelease *Release

	rv := Changelog{}
	lastDetail := ""
	commentBlock := false
	phase := headerBlock
	line := 0

	s := bufio.NewScanner(r)

	for s.Scan() {
		text := s.Text()
		line++

		// Skip empty lines everywhere except the title block
		if phase != titleBlock && emptyLine.MatchString(text) {
			continue
		}

		// The bufio.Scanner doesn't allow for backtracking.  To simplify the
		// overall logic, using a goto to jump to the right parsing section
		// logic is probably the easiest thing to do right now.
	reevaluate_line:

		switch phase {
		case headerBlock:
			// The comment handling is pretty weak.  TODO: handle multiple
			// comments per line or other weird variants
			if commentStart.MatchString(text) {
				commentBlock = true
			}

			if commentBlock {
				rv.CommentHeader = append(rv.CommentHeader, text)
				if commentEnd.MatchString(text) {
					commentBlock = false
				}
				if commentBlock {
					continue
				}
			}

			if titleRE.MatchString(text) {
				title := titleRE.FindStringSubmatch(text)
				if 1 <= len(title) {
					rv.Title = title[1]
				} else {
					return nil, fmt.Errorf("Invalid title found at line: %d", line)
				}
				phase = titleBlock
			}

		case titleBlock:
			if releaseRE.MatchString(text) {
				rv.evalDesc()
				phase = releasesBlock
				goto reevaluate_line
			}
			rv.Description = append(rv.Description, text)

		case releasesBlock:
			if linksRE.MatchString(text) {
				if lastRelease != nil {
					rv.Releases = append(rv.Releases, *lastRelease)
					lastRelease = nil
				}

				phase = linksBlock
				goto reevaluate_line
			}

			if releaseRE.MatchString(text) {
				if lastRelease != nil {
					rv.Releases = append(rv.Releases, *lastRelease)
				}
				lastRelease = new(Release)

				// Always add the line to the enire body field
				lastRelease.Body = append(lastRelease.Body, text)

				found := releaseRE.FindStringSubmatch(text)
				if 1 <= len(found) {
					lastRelease.Title = found[1]
				}
				if 2 <= len(found) {
					lastRelease.Version = found[2]
				}
				if 4 <= len(found) && found[4] != "" {
					got, err := time.Parse("2006-01-02", found[4])
					if nil != err {
						return nil, fmt.Errorf("Invalid date found at line: %d.  YYYY-MM-DD is required.  '%s' found.", line, found[4])
					}
					lastRelease.Date = &got
				}
				if 5 <= len(found) && found[5] != "" {
					lastRelease.Yanked = true
				}
				lastDetail = ""
				continue
			}

			// Always add the line to the enire body field
			lastRelease.Body = append(lastRelease.Body, text)

			if detailsRE.MatchString(text) {
				found := detailsRE.FindStringSubmatch(text)
				if 1 <= len(found) {
					lastDetail = strings.ToLower(found[1])
				}
				continue
			}

			switch lastDetail {
			case "":
				lastRelease.Other = append(lastRelease.Other, text)
			case "added":
				lastRelease.Added = append(lastRelease.Added, text)
			case "changed":
				lastRelease.Changed = append(lastRelease.Changed, text)
			case "deprecated":
				lastRelease.Deprecated = append(lastRelease.Deprecated, text)
			case "fixed":
				lastRelease.Fixed = append(lastRelease.Fixed, text)
			case "removed":
				lastRelease.Removed = append(lastRelease.Removed, text)
			case "security":
				lastRelease.Security = append(lastRelease.Security, text)
			}

		case linksBlock:
			if linksRE.MatchString(text) {
				found := linksRE.FindStringSubmatch(text)
				if 2 <= len(found) {
					link := Link{
						Version: found[1],
						Url:     found[2],
					}
					rv.Links = append(rv.Links, link)
				}
			}
		}
	}

	// There could be a last release if there was no links section
	if lastRelease != nil {
		rv.Releases = append(rv.Releases, *lastRelease)
		lastRelease = nil
	}

	if err := s.Err(); err != nil {
		return nil, err
	}

	return &rv, nil
}

// ToMarkdown converts the Changelog structure into a markdown formatted stream of
// characters and returns the string.
func (cl *Changelog) ToMarkdown() string {
	out := ""
	for _, line := range cl.CommentHeader {
		out += line + "\n"
	}

	out += "# " + cl.Title + "\n\n"

	for _, line := range cl.Description {
		out += line + "\n"
	}

	for _, r := range cl.Releases {
		out += "\n\n" + r.ToMarkdown()
	}

	if 0 < len(cl.Links) {
		out += "\n\n"
		for _, link := range cl.Links {
			out += link.ToMarkdown()
		}
	}

	return out
}

// evalDesc looks at the description and finds if there are versions for the
// semver or for keep a changelog version and populates that information.
func (cl *Changelog) evalDesc() {
	desc := strings.Join(cl.Description, " ")

	semver := semverRE.FindStringSubmatch(desc)
	if 1 <= len(semver) {
		cl.SemVerVersion = semver[1]
	}

	clver := changelogVerRE.FindStringSubmatch(desc)
	if 1 <= len(clver) {
		cl.KeepAChangelogVersion = clver[1]
	}
}

// ToMarkdown converts the Release structure into a markdown formatted stream of
// characters and returns the string.
func (r *Release) ToMarkdown() string {
	out := "## [" + r.Version + "]"

	if r.Date != nil {
		out += " - " + r.Date.Format("2006-01-02")
	}

	if r.Yanked {
		out += " [YANKED]"
	}

	out += "\n"

	type List struct {
		sect   []string
		header string
	}

	list := []List{
		{r.Other, ""},
		{r.Added, "Added"},
		{r.Changed, "Changed"},
		{r.Deprecated, "Deprecated"},
		{r.Fixed, "Fixed"},
		{r.Removed, "Removed"},
		{r.Security, "Security"},
	}

	for _, section := range list {
		out += func(sec List) string {
			out := ""
			if 0 < len(sec.sect) {
				if sec.header != "" {
					out += "\n### " + sec.header + "\n"
				}

				for _, line := range sec.sect {
					out += line + "\n"
				}
			}

			return out
		}(section)
	}

	return out
}

// ToMarkdown converts the Link structure into a markdown formatted stream of
// characters and returns the string.
func (l *Link) ToMarkdown() string {
	return fmt.Sprintf("[%s]: %s\n", l.Version, l.Url)
}
