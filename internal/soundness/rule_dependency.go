package soundness

import "strings"

const (
	ruleDependencyConsistency = "dependency-consistency"

	// maxCycleFindings bounds cycle findings; further cycles are summarized.
	maxCycleFindings = 5
	// maxCyclePathIDs bounds the IDs printed for one cycle path.
	maxCyclePathIDs = 10
)

// checkDependencyConsistency flags dangling and self dependency edges and
// dependency cycles (one finding per back edge, so distinct cycles that share
// a back edge are reported once). Cycle findings are capped and their printed
// paths truncated so a pathological graph cannot produce unbounded output.
// Repeated identical dangling or self entries on one unit are reported once.
func checkDependencyConsistency(ctx *checkContext) ([]Finding, error) {
	var out []Finding
	for _, u := range ctx.plan.Units {
		seen := map[string]bool{}
		for _, dep := range u.Dependencies {
			if seen[dep] {
				continue
			}
			seen[dep] = true
			switch {
			case dep == u.ID:
				out = append(out, errFinding(ruleDependencyConsistency, u.ID, "unit depends on itself"))
			case ctx.byID[dep] == nil:
				out = append(out, errFinding(ruleDependencyConsistency, u.ID, "dangling dependency %q: no such unit", dep))
			}
		}
	}

	const (
		unvisited = iota
		inProgress
		done
	)
	state := map[string]int{}
	cycles, omitted := 0, 0
	type frame struct {
		id   string
		next int
	}
	for _, root := range ctx.plan.Units {
		if ctx.byID[root.ID] == nil || state[root.ID] != unvisited {
			continue
		}
		state[root.ID] = inProgress
		stack := []frame{{id: root.ID}}
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			deps := ctx.byID[top.id].Dependencies
			if top.next >= len(deps) {
				state[top.id] = done
				stack = stack[:len(stack)-1]
				continue
			}
			dep := deps[top.next]
			top.next++
			if dep == top.id || ctx.byID[dep] == nil {
				continue
			}
			switch state[dep] {
			case unvisited:
				state[dep] = inProgress
				stack = append(stack, frame{id: dep})
			case inProgress:
				if cycles >= maxCycleFindings {
					omitted++
					continue
				}
				cycles++
				start := 0
				for i, f := range stack {
					if f.id == dep {
						start = i
						break
					}
				}
				ids := make([]string, 0, len(stack)-start+1)
				for _, f := range stack[start:] {
					ids = append(ids, f.id)
				}
				ids = append(ids, dep)
				out = append(out, errFinding(ruleDependencyConsistency, dep, "dependency cycle: %s", cyclePath(ids)))
			}
		}
	}
	if omitted > 0 {
		out = append(out, errFinding(ruleDependencyConsistency, "", "additional cycles omitted: %d more dependency cycle(s) not listed", omitted))
	}
	return out, nil
}

// cyclePath renders a cycle path, truncating after maxCyclePathIDs IDs.
func cyclePath(ids []string) string {
	if len(ids) <= maxCyclePathIDs {
		return strings.Join(ids, " -> ")
	}
	return strings.Join(ids[:maxCyclePathIDs], " -> ") + " -> …"
}
