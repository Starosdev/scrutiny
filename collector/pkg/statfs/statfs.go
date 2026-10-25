// Package statfs reports filesystem capacity the way df does.
package statfs

// Result holds statfs(2) figures in bytes. Used is Blocks-Bfree and Available is Bavail, so
// Used/(Used+Available) is df's Use% on any filesystem, including Btrfs with mirrored profiles.
type Result struct {
	Total     int64
	Used      int64
	Available int64
}
