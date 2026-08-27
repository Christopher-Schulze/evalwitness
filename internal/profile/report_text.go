package profile

import (
	"fmt"
	"sort"
	"strings"
)

// TextReport is concise text directly from profile with sorted dimensions.
func TextReport(p Profile) string {
	ids := make([]string, 0, len(p.Dimensions))
	for _, d := range p.Dimensions {
		ids = append(ids, string(d.ID)+":"+string(d.Status))
	}
	sort.Strings(ids)
	return fmt.Sprintf("Profile %s %d dims digest %s [%s]", p.Identity, len(p.Dimensions), p.Digest, strings.Join(ids, ","))
}

// MarkdownReport is structured markdown directly from profile with evidence table.
func MarkdownReport(p Profile) string {
	var b strings.Builder
	if _, err := fmt.Fprintf(&b, "# Profile %s\n\n- Dimensions: %d\n- Digest: %s\n- Route: %s\n- Protocol: %s\n\n| Dimension | Status | Metric | Scope |\n|---|---|---|---|\n", p.Identity, len(p.Dimensions), p.Digest, p.RouteScope, p.ProtocolVersion); err != nil {
		return ""
	}
	dims := append([]Dimension(nil), p.Dimensions...)
	sort.Slice(dims, func(i, j int) bool { return string(dims[i].ID) < string(dims[j].ID) })
	for _, d := range dims {
		metric := "—"
		if d.Metric != nil {
			metric = *d.Metric
		}
		if _, err := fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", d.ID, d.Status, metric, d.Scope); err != nil {
			return ""
		}
	}
	return b.String()
}
