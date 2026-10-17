package evidencedag

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const truncationMarker = "\n... truncated\n"

// boundOutput caps rendered output at MaxReportBytes on a UTF-8 boundary.
func boundOutput(s string) string {
	if len(s) <= MaxReportBytes {
		return s
	}
	cut := MaxReportBytes - len(truncationMarker)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + truncationMarker
}

// Text renders a deterministic, bounded status report: roll-up first, then
// every node in ID order.
func (r Report) Text() string {
	ru := r.Rollup()
	var b strings.Builder
	fmt.Fprintf(&b, "shipment: %s\nhead: %s\n", truncate(r.ShipmentID), truncate(r.HeadCommit))
	fmt.Fprintf(&b, "rollup: %s (%s)\n", ru.State, ru.Reason)
	fmt.Fprintf(&b, "applicable terminals: %d\n", ru.ApplicableTerminals)
	for _, l := range []struct {
		name string
		ids  []string
	}{{"unmet", ru.Unmet}, {"unverifiable", ru.Unverifiable}, {"advisory", ru.Advisory}} {
		if len(l.ids) > 0 {
			fmt.Fprintf(&b, "%s: %s\n", l.name, strings.Join(l.ids, ", "))
		}
	}
	b.WriteString("nodes:\n")
	for _, n := range r.Nodes {
		fmt.Fprintf(&b, "  %s\t%s\t%s\tterminal=%t\t%s\n", truncate(n.ID), n.Status, truncate(string(n.Family)), n.Terminal, truncate(n.Reason))
	}
	return boundOutput(b.String())
}

func dotEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(truncate(s))
}

func dotQuote(s string) string { return `"` + dotEscape(s) + `"` }

var dotColors = map[Status]string{
	StatusVerified: "green", StatusWaived: "blue", StatusUnmet: "red",
	StatusError: "orange", StatusNotApplicable: "gray",
}

// DOT renders a deterministic, bounded Graphviz graph of the declaration.
// Edges point from a node to the node it depends on. When r is non-nil, each
// node is labelled and coloured with its evaluated status.
func (d *Declaration) DOT(r *Report) string {
	var b strings.Builder
	b.WriteString("digraph evidence {\n  rankdir=LR;\n")
	nodes := append([]Node(nil), d.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	for _, n := range nodes {
		label := dotEscape(n.ID)
		attrs := ""
		if r != nil {
			if res, ok := r.Node(n.ID); ok {
				label += `\n` + dotEscape(string(res.Status))
				attrs = fmt.Sprintf(", color=%s", dotColors[res.Status])
			}
		}
		shape := "box"
		if n.Terminal {
			shape = "doubleoctagon"
		}
		fmt.Fprintf(&b, "  %s [label=%s, shape=%s%s];\n", dotQuote(n.ID), `"`+label+`"`, shape, attrs)
	}
	edges := append([]Edge(nil), d.Edges...)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Node != edges[j].Node {
			return edges[i].Node < edges[j].Node
		}
		return edges[i].DependsOn < edges[j].DependsOn
	})
	for _, e := range edges {
		fmt.Fprintf(&b, "  %s -> %s;\n", dotQuote(e.Node), dotQuote(e.DependsOn))
	}
	b.WriteString("}\n")
	return boundOutput(b.String())
}
