// Package gover provides operations on [Go versions]
// in [Go toolchain name syntax]: strings like
// "go1.20", "go1.21.0", "go1.22rc2", and "go1.23.4-bigcorp".
//
// [Go versions]: https://go.dev/doc/toolchain#version
// [Go toolchain name syntax]: https://go.dev/doc/toolchain#name
package gover // import "go/version"

import (
	"cmp"
	"fmt"
	"strings"
)

// stripGo converts from a "go1.21-bigcorp" version to a "1.21" version.
// If v does not start with "go", stripGo returns the empty string (a known invalid version).
func stripGo(v string) string {
	v, _, _ = strings.Cut(v, "-") // strip -bigcorp suffix.
	if len(v) < 2 || v[:2] != "go" {
		return ""
	}
	// trim prefix "go"
	return v[2:]
}

// Lang returns the Go language version for version x.
// If x is not a valid version, Lang returns the empty string.
// For example:
//
//	Lang("go1.21rc2") = "go1.21"
//	Lang("go1.21.2") = "go1.21"
//	Lang("go1.21") = "go1.21"
//	Lang("go1") = "go1"
//	Lang("bad") = ""
//	Lang("1.21") = ""
func Lang(x string) string {
	v := lang(stripGo(x))
	if v == "" {
		return ""
	}
	if strings.HasPrefix(x[2:], v) {
		return x[:2+len(v)] // "go"+v without allocation
	} else {
		return "go" + v
	}
}

func Parse(x string) (Version, error) {
	parsedV := parse(stripGo(x))
	if (parsedV == Version{}) {
		return Version{}, fmt.Errorf("invalid version %s", x)
	}
	return parsedV, nil
}

// Compare returns -1, 0, or +1 depending on whether
// x < y, x == y, or x > y, interpreted as Go versions.
// The versions x and y must begin with a "go" prefix: "go1.21" not "1.21".
// Invalid versions, including the empty string, compare less than
// valid versions and equal to each other.
// After go1.21, the language version is less than specific release versions
// or other prerelease versions.
// For example:
//
//	Compare("go1.21rc1", "go1.21") = 1
//	Compare("go1.21rc1", "go1.21.0") = -1
//	Compare("go1.22rc1", "go1.22") = 1
//	Compare("go1.22rc1", "go1.22.0") = -1
//
// However, When the language version is below go1.21, the situation is quite different,
// because the initial release version was 1.N, not 1.N.0.
// For example:
//
//	Compare("go1.20rc1", "go1.21") = -1
//	Compare("go1.19rc1", "go1.19") = -1
//	Compare("go1.18", "go1.18rc1") = 1
//	Compare("go1.18", "go1.18rc1") = 1
//
// This situation also happens to prerelease for some old patch versions, such as "go1.8.5rc5, "go1.9.2rc2"
// For example:
//
//	Compare("go1.8.5rc4", "go1.8.5rc5") = -1
//	Compare("go1.8.5rc5", "go1.8.5") = -1
//	Compare("go1.9.2rc2", "go1.9.2") = -1
//	Compare("go1.9.2rc2", "go1.9") = 1
func Compare(x, y string) int {
	return compare(stripGo(x), stripGo(y))
}

// IsValid reports whether the version x is valid.
func IsValid(x string) bool {
	return isValid(stripGo(x))
}

// A Version is a parsed Go version: major[.Minor[.Patch]][kind[pre]]
// The numbers are the original decimal strings to avoid integer overflows
// and since there is very little actual math. (Probably overflow doesn't matter in practice,
// but at the time this code was written, there was an existing test that used
// go1.99999999999, which does not fit in an int on 32-bit platforms.
// The "big decimal" representation avoids the problem entirely.)
type Version struct {
	Major string // decimal
	Minor string // decimal or ""
	Patch string // decimal or ""
	Kind  string // "", "alpha", "beta", "rc"
	Pre   string // decimal or ""
}

func (v Version) String() string {
	return fmt.Sprintf("go%s.%s.%s%s%s", v.Major, v.Minor, v.Patch, v.Kind, v.Pre)
}

// Compare returns -1, 0, or +1 depending on whether
// x < y, x == y, or x > y, interpreted as toolchain versions.
// The versions x and y must not begin with a "go" prefix: just "1.21" not "go1.21".
// Malformed versions compare less than well-formed versions and equal to each other.
// The language version "1.21" compares less than the release candidate and eventual releases "1.21rc1" and "1.21.0".
func compare(x, y string) int {
	vx := parse(x)
	vy := parse(y)

	if c := CmpInt(vx.Major, vy.Major); c != 0 {
		return c
	}
	if c := CmpInt(vx.Minor, vy.Minor); c != 0 {
		return c
	}
	if c := CmpInt(vx.Patch, vy.Patch); c != 0 {
		return c
	}
	if c := cmp.Compare(vx.Kind, vy.Kind); c != 0 { // "" < alpha < beta < rc
		// for patch release, alpha < beta < rc < ""
		if vx.Patch != "" {
			if vx.Kind == "" {
				c = 1
			} else if vy.Kind == "" {
				c = -1
			}
		}
		return c
	}
	if c := CmpInt(vx.Pre, vy.Pre); c != 0 {
		return c
	}
	return 0
}

// Max returns the maximum of x and y interpreted as toolchain versions,
// compared using Compare.
// If x and y compare equal, Max returns x.
func Max(x, y string) string {
	if Compare(x, y) < 0 {
		return y
	}
	return x
}

// isLang reports whether v denotes the overall Go language version
// and not a specific release. Starting with the Go 1.21 release, "1.x" denotes
// the overall language version; the first release is "1.x.0".
// The distinction is important because the relative ordering is
//
//	1.21 < 1.21rc1 < 1.21.0
//
// meaning that Go 1.21rc1 and Go 1.21.0 will both handle go.mod files that
// say "go 1.21", but Go 1.21rc1 will not handle files that say "go 1.21.0".
func isLang(x string) bool {
	v := parse(x)
	return v != Version{} && v.Patch == "" && v.Kind == "" && v.Pre == ""
}

// lang returns the Go language version. For example, Lang("1.2.3") == "1.2".
func lang(x string) string {
	v := parse(x)
	if v.Minor == "" || v.Major == "1" && v.Minor == "0" {
		return v.Major
	}
	return v.Major + "." + v.Minor
}

// isValid reports whether the version x is valid.
func isValid(x string) bool {
	return parse(x) != Version{}
}

// parse parses the Go version string x into a version.
// It returns the zero version if x is malformed.
func parse(x string) Version {
	var v Version

	// Parse major version.
	var ok bool
	v.Major, x, ok = cutInt(x)
	if !ok {
		return Version{}
	}
	if x == "" {
		// Interpret "1" as "1.0.0".
		v.Minor = "0"
		v.Patch = "0"
		return v
	}

	// Parse . before minor version.
	if x[0] != '.' {
		return Version{}
	}

	// Parse minor version.
	v.Minor, x, ok = cutInt(x[1:])
	if !ok {
		return Version{}
	}
	if x == "" {
		// Patch missing is same as "0" for older versions.
		// Starting in Go 1.21, patch missing is different from explicit .0.
		if CmpInt(v.Minor, "21") < 0 {
			v.Patch = "0"
		}
		return v
	}

	// Parse patch if present.
	if x[0] == '.' {
		v.Patch, x, ok = cutInt(x[1:])
		if !ok {
			return Version{}
		}

		// If there has prerelease for patch releases.
		if x != "" {
			v.Kind, v.Pre, ok = parsePreRelease(x)
			if !ok {
				return Version{}
			}
		}

		return v
	}

	// Parse prerelease.
	v.Kind, v.Pre, ok = parsePreRelease(x)
	if !ok {
		return Version{}
	}
	return v
}

func parsePreRelease(x string) (kind, pre string, ok bool) {
	i := 0
	for i < len(x) && (x[i] < '0' || '9' < x[i]) {
		if x[i] < 'a' || 'z' < x[i] {
			return "", "", false
		}
		i++
	}
	if i == 0 {
		return "", "", false
	}
	kind, x = x[:i], x[i:]
	if x == "" {
		return kind, "", true
	}
	pre, x, ok = cutInt(x)
	if !ok || x != "" {
		return "", "", false
	}
	return kind, pre, true
}

// cutInt scans the leading decimal number at the start of x to an integer
// and returns that value and the rest of the string.
func cutInt(x string) (n, rest string, ok bool) {
	i := 0
	for i < len(x) && '0' <= x[i] && x[i] <= '9' {
		i++
	}
	if i == 0 || x[0] == '0' && i != 1 { // no digits or unnecessary leading zero
		return "", "", false
	}
	return x[:i], x[i:], true
}

// CmpInt returns cmp.Compare(x, y) interpreting x and y as decimal numbers.
// (Copied from golang.org/x/mod/semver's compareInt.)
func CmpInt(x, y string) int {
	if x == y {
		return 0
	}
	if len(x) < len(y) {
		return -1
	}
	if len(x) > len(y) {
		return +1
	}
	if x < y {
		return -1
	} else {
		return +1
	}
}

// DecInt returns the decimal string decremented by 1, or the empty string
// if the decimal is all zeroes.
// (Copied from golang.org/x/mod/module's decDecimal.)
func DecInt(decimal string) string {
	// Scan right to left turning 0s to 9s until you find a digit to decrement.
	digits := []byte(decimal)
	i := len(digits) - 1
	for ; i >= 0 && digits[i] == '0'; i-- {
		digits[i] = '9'
	}
	if i < 0 {
		// decimal is all zeros
		return ""
	}
	if i == 0 && digits[i] == '1' && len(digits) > 1 {
		digits = digits[1:]
	} else {
		digits[i]--
	}
	return string(digits)
}
