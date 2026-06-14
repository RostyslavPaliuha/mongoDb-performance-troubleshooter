package mongodb

import (
	"fmt"
	"strconv"
	"strings"
)

type Version struct {
	Major int
	Minor int
	Patch int
}

func ParseVersion(raw string) (Version, error) {
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		return Version{}, fmt.Errorf("invalid MongoDB version %q", raw)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid MongoDB major version %q: %w", raw, err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("invalid MongoDB minor version %q: %w", raw, err)
	}

	patch := 0
	if len(parts) >= 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return Version{}, fmt.Errorf("invalid MongoDB patch version %q: %w", raw, err)
		}
	}

	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

func (v Version) AtLeast(major, minor, patch int) bool {
	if v.Major != major {
		return v.Major > major
	}
	if v.Minor != minor {
		return v.Minor > minor
	}
	return v.Patch >= patch
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}
