/**
 *  Copyright (c) 2021  Comcast Cable Communications Management, LLC
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
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func getStrict() io.Reader {
	body := `
<!--
SPDX-FileCopyrightText: 2021 Comcast Cable Communications Management, LLC
SPDX-License-Identifier: Apache-2.0
-->
# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v3.4.0]

- Something that doesn't fit below.

### Added
- Added a new string.
- Added a new line.

### Changed
- Allow use of num_algorithms.
- A few lines related to the ### Fixed field

### Fixed
- Fixed [issue 55](https://example.com/issue-55)

### Security

- Fixed a buffer overrun issue-1234

### Changed
- I forgot to include this above

## [v3.0.0] - 2020-12-30

### Deprecated
- The Magic() function has been deprecated.

### Removed
- The ReallyMagic() function has been deprecated.

## [v2.1.0] - 2019-12-30 [YANKED]

## [v2.0.0] [YANKED]

[Unreleased]: https://example.com/compare/v3.4.0...HEAD
[v3.4.0]: https://example.com/compare/v3.0.0...v3.4.0
[v3.0.0]: https://example.com/compare/v0.0.0...v3.4.0
`
	var rv io.Reader = strings.NewReader(body)

	return rv
}

func getMarkdownFromStrict() string {
	body := `<!--
SPDX-FileCopyrightText: 2021 Comcast Cable Communications Management, LLC
SPDX-License-Identifier: Apache-2.0
-->
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).



## [Unreleased]


## [v3.4.0]
- Something that doesn't fit below.

### Added
- Added a new string.
- Added a new line.

### Changed
- Allow use of num_algorithms.
- A few lines related to the ### Fixed field
- I forgot to include this above

### Fixed
- Fixed [issue 55](https://example.com/issue-55)

### Security
- Fixed a buffer overrun issue-1234


## [v3.0.0] - 2020-12-30

### Deprecated
- The Magic() function has been deprecated.

### Removed
- The ReallyMagic() function has been deprecated.


## [v2.1.0] - 2019-12-30 [YANKED]


## [v2.0.0] [YANKED]


[Unreleased]: https://example.com/compare/v3.4.0...HEAD
[v3.4.0]: https://example.com/compare/v3.0.0...v3.4.0
[v3.0.0]: https://example.com/compare/v0.0.0...v3.4.0
`

	return body
}

func TestParseStrict(t *testing.T) {
	assert := assert.New(t)

	rv, err := Parse(getStrict(), nil)

	assert.NotNil(rv)
	assert.Nil(err)

	// Spot check the values being returned for some sanity
	assert.Equal("1.0.0", rv.KeepAChangelogVersion)
	assert.Equal("v2.0.0", rv.SemVerVersion)

	assert.Equal(4, len(rv.CommentHeader))
	assert.Equal("<!--", rv.CommentHeader[0])
	assert.Equal("SPDX-FileCopyrightText: 2021 Comcast Cable Communications Management, LLC", rv.CommentHeader[1])
	assert.Equal("SPDX-License-Identifier: Apache-2.0", rv.CommentHeader[2])
	assert.Equal("-->", rv.CommentHeader[3])

	assert.Equal("Changelog", rv.Title)
	assert.Equal(5, len(rv.Description))
	assert.Equal(5, len(rv.Releases))
	assert.Equal(3, len(rv.Links))

	r := rv.Releases[0]
	assert.Equal("[Unreleased]", r.Title)
	assert.Equal("Unreleased", r.Version)
	assert.Nil(r.Date)

	r = rv.Releases[1]
	assert.Equal("[v3.4.0]", r.Title)
	assert.Equal("v3.4.0", r.Version)
	assert.Nil(r.Date)
	assert.Equal(false, r.Yanked)
	assert.Equal(14, len(r.Body))
	assert.Equal(1, len(r.Other))
	assert.Equal(2, len(r.Added))
	assert.Equal(3, len(r.Changed))
	assert.Equal(1, len(r.Fixed))
	assert.Equal(1, len(r.Security))
	assert.Equal(0, len(r.Removed))
	assert.Equal(0, len(r.Deprecated))

	r = rv.Releases[2]
	assert.Equal("[v3.0.0] - 2020-12-30", r.Title)
	assert.Equal("v3.0.0", r.Version)
	assert.NotNil(r.Date)
	assert.Equal(1, len(r.Removed))
	assert.Equal(1, len(r.Deprecated))

	r = rv.Releases[3]
	assert.Equal("[v2.1.0] - 2019-12-30 [YANKED]", r.Title)
	assert.Equal("v2.1.0", r.Version)
	assert.NotNil(r.Date)
	assert.Equal(true, r.Yanked)

	r = rv.Releases[4]
	assert.Equal("[v2.0.0] [YANKED]", r.Title)
	assert.Equal("v2.0.0", r.Version)
	assert.Nil(r.Date)
	assert.Equal(true, r.Yanked)

	assert.Equal(3, len(rv.Links))
}

func TestToMarkdown(t *testing.T) {
	assert := assert.New(t)

	rv, err := Parse(getStrict(), nil)

	assert.NotNil(rv)
	assert.Nil(err)

	got := rv.ToMarkdown()

	assert.Equal(getMarkdownFromStrict(), got)
}

func TestParseShort(t *testing.T) {
	body := `
# Valid but different
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
- Something new but unreleased
`
	var bodyReader io.Reader = strings.NewReader(body)

	assert := assert.New(t)

	rv, err := Parse(bodyReader, nil)

	assert.NotNil(rv)
	assert.Nil(err)

	// Spot check the values being returned for some sanity
	assert.Equal(0, len(rv.CommentHeader))

	assert.Equal("Valid but different", rv.Title)
	assert.Equal(1, len(rv.Releases))
	assert.Equal(0, len(rv.Links))

	r := rv.Releases[0]
	assert.Equal(1, len(r.Other))
}
