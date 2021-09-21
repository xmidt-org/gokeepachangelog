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

var (
	emptyLine      = regexp.MustCompile(`^\s*$`)
	commentCheck   = regexp.MustCompile(`^\s*<!--.*-->\s*$`)
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

	rv := Changelog{}

	s := bufio.NewScanner(r)

	err := rv.addHeaders(s)
	if err != nil {
		return nil, err
	}

	rv.addTitleBlock(s)
	err = rv.addReleases(s)
	if err != nil {
		return nil, err
	}
	rv.addLinks(s)

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

// newRelease attempts to create a new release object based off the stream of
// data from the scanner.  When it returns (nil, nil) there is nothing left to
// do and there are no more releases
func newRelease(s *bufio.Scanner) (*Release, error) {
	if !releaseRE.MatchString(s.Text()) {
		return nil, nil
	}

	r := new(Release)

	r.Body = append(r.Body, s.Text())

	found := releaseRE.FindStringSubmatch(s.Text())
	r.Title = found[1]
	r.Version = found[2]
	if found[4] != "" {
		got, err := time.Parse("2006-01-02", found[4])
		if nil != err {
			return nil, fmt.Errorf("Invalid date found: '%s'.  YYYY-MM-DD is required.", found[4])
		}
		r.Date = &got
	}
	if found[5] != "" {
		r.Yanked = true
	}

	lastDetail := ""
	for {
		if s.Scan() == false ||
			linksRE.MatchString(s.Text()) ||
			releaseRE.MatchString(s.Text()) {
			return r, nil
		}

		text := s.Text()
		if emptyLine.MatchString(text) {
			continue
		}

		r.Body = append(r.Body, text)

		if detailsRE.MatchString(text) {
			found := detailsRE.FindStringSubmatch(text)
			if 1 <= len(found) {
				lastDetail = strings.ToLower(found[1])
			}
			continue
		}

		switch lastDetail {
		case "":
			r.Other = append(r.Other, text)
		case "added":
			r.Added = append(r.Added, text)
		case "changed":
			r.Changed = append(r.Changed, text)
		case "deprecated":
			r.Deprecated = append(r.Deprecated, text)
		case "fixed":
			r.Fixed = append(r.Fixed, text)
		case "removed":
			r.Removed = append(r.Removed, text)
		case "security":
			r.Security = append(r.Security, text)
		}
	}
}

// addHeaders adds the header comments if present to the changelog object.
func (cl *Changelog) addHeaders(s *bufio.Scanner) error {
	for {
		if titleRE.MatchString(s.Text()) {
			full := strings.Join(cl.CommentHeader, " ")
			if full != "" && false == commentCheck.MatchString(full) {
				return fmt.Errorf("Header was not just comments.")
			}
			return nil
		}

		if !emptyLine.MatchString(s.Text()) {
			cl.CommentHeader = append(cl.CommentHeader, s.Text())
		}

		if s.Scan() == false {
			return fmt.Errorf("Only the header was present.")
		}
	}
}

// addReleases adds all the found releases to the changelog object.
func (cl *Changelog) addReleases(s *bufio.Scanner) error {
	for {
		r, err := newRelease(s)
		if err != nil {
			return err
		}
		if r == nil {
			return nil
		}

		cl.Releases = append(cl.Releases, *r)
	}
}

// addTitleBlock adds the title block information to the changelog object.
func (cl *Changelog) addTitleBlock(s *bufio.Scanner) {
	title := titleRE.FindStringSubmatch(s.Text())
	if title == nil {
		return
	}

	cl.Title = title[1]

	for {
		if s.Scan() == false ||
			linksRE.MatchString(s.Text()) ||
			releaseRE.MatchString(s.Text()) {
			cl.evalDesc()
			return
		}

		cl.Description = append(cl.Description, s.Text())
	}
}

// addLinks adds the links (if present) to the changelog object.
func (cl *Changelog) addLinks(s *bufio.Scanner) {
	for {
		found := linksRE.FindStringSubmatch(s.Text())
		if found != nil {
			link := Link{
				Version: found[1],
				Url:     found[2],
			}
			cl.Links = append(cl.Links, link)
		}

		if !s.Scan() {
			return
		}
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
