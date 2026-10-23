package tui

import (
	"sort"
	"strings"

	"github.com/sahilm/fuzzy"
)

// Row is one entry in a sidebar list.
type Row struct {
	ID     string
	N      int
	Label  string
	Pinned bool
	Note   string
}

// FilterRows keeps rows that match query. With an empty query, pinned rows
// stay first and the relative order of the others is preserved. A non-empty
// query ranks by fuzzy score, and a pinned row wins a tie.
func FilterRows(rows []Row, query string) []Row {
	query = strings.TrimSpace(query)
	if query == "" {
		out := append([]Row(nil), rows...)
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Pinned == out[j].Pinned {
				return false
			}
			return out[i].Pinned
		})
		return out
	}
	type scored struct {
		row   Row
		score int
	}
	matched := make([]scored, 0, len(rows))
	for _, row := range rows {
		matches := fuzzy.Find(query, []string{row.Label})
		if len(matches) == 0 {
			continue
		}
		matched = append(matched, scored{row: row, score: matches[0].Score})
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].score != matched[j].score {
			return matched[i].score > matched[j].score
		}
		if matched[i].row.Pinned != matched[j].row.Pinned {
			return matched[i].row.Pinned
		}
		return false
	})
	out := make([]Row, len(matched))
	for i := range matched {
		out[i] = matched[i].row
	}
	return out
}
