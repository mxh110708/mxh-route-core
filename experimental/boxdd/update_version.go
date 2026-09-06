package main

import "golang.org/x/mod/semver"

// Compare the full desktop SemVer, including nested beta and custom revisions.
func isNewerDesktopVersion(candidate, installed string) bool {
	left, right := "v"+candidate, "v"+installed
	return semver.IsValid(left) && semver.IsValid(right) && semver.Compare(left, right) > 0
}
