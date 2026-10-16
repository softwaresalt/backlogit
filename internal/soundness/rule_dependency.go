package soundness

import "strings"

const ruleDependencyConsistency = "dependency-consistency"

// checkDependencyConsistency flags dangling and self dependency edges and
// dependency cycles (one finding per cycle).
func checkDependencyConsistency(ctx *checkContext) ([]Finding, error) {
	var out []Finding
	for _, u := range ctx.plan.Units {
		for _, dep := range u.Dependencies {
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
	var stack []string
	var visit func(id string)
	visit = func(id string) {
		state[id] = inProgress
		stack = append(stack, id)
		for _, dep := range ctx.byID[id].Dependencies {
			if dep == id || ctx.byID[dep] == nil {
				continue
			}
			switch state[dep] {
			case unvisited:
				visit(dep)
			case inProgress:
				start := 0
				for i, s := range stack {
					if s == dep {
						start = i
						break
					}
				}
				cycle := append(append([]string{}, stack[start:]...), dep)
				out = append(out, errFinding(ruleDependencyConsistency, dep, "dependency cycle: %s", strings.Join(cycle, " -> ")))
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = done
	}
	for _, u := range ctx.plan.Units {
		if ctx.byID[u.ID] != nil && state[u.ID] == unvisited {
			visit(u.ID)
		}
	}
	return out, nil
}
