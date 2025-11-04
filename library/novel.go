package library

import (
	"time"

	"novel_reader/lang"
)

type Novel struct {
	Name      string
	Author    string
	Path      string
	Latest    string
	Current   string
	Modified  time.Time // last read or last modified
	Added     time.Time // when the novel file/cache was created or detected
	OnlineURL string    // optional, empty if local
	IsLocal   bool
}

// list.Item interface for Bubble Tea
func (n Novel) Title() string { return n.Name }
func (n Novel) Description() string {
	if n.IsLocal {
		return lang.Active().Novel.LocalPrefix + " | " + n.Latest
	}
	return n.Author + " | " + n.Latest
}
func (n Novel) FilterValue() string { return n.Name + " | " + n.Author }
