package evidencedag

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"sort"
	"sync"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

type cacheEntry struct {
	key    string
	result NodeResult
}

// Evaluator evaluates one declaration for any number of shipments. It caches
// per-node results keyed by a content digest of the node, its resolved inputs
// and the keys of all its dependencies, so only affected nodes are recomputed.
// It holds no global state and is safe for concurrent use.
type Evaluator struct {
	nodes map[string]Node
	deps  map[string][]string
	order []string

	mu    sync.Mutex
	cache map[string]cacheEntry
}

// NewEvaluator validates d and returns an Evaluator over a private copy.
func NewEvaluator(d *Declaration) (*Evaluator, error) {
	if d == nil {
		return nil, fmt.Errorf("%w: nil declaration", ErrInvalidDeclaration)
	}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	e := &Evaluator{nodes: make(map[string]Node, len(d.Nodes)), deps: map[string][]string{}, cache: map[string]cacheEntry{}}
	for _, n := range d.Nodes {
		e.nodes[n.ID] = cloneNode(n)
	}
	for _, ed := range d.Edges {
		e.deps[ed.Node] = append(e.deps[ed.Node], ed.DependsOn)
	}
	for k := range e.deps {
		sort.Strings(e.deps[k])
	}
	e.order = topoOrder(e.nodes, e.deps)
	return e, nil
}

func cloneNode(n Node) Node {
	if n.Applicability != nil {
		a := &Applicability{Always: n.Applicability.Always}
		if n.Applicability.Requires != nil {
			a.Requires = make(map[string][]string, len(n.Applicability.Requires))
			for k, v := range n.Applicability.Requires {
				a.Requires[k] = append([]string(nil), v...)
			}
		}
		n.Applicability = a
	}
	return n
}

// topoOrder returns dependencies-first order, ties broken by node ID.
func topoOrder(nodes map[string]Node, deps map[string][]string) []string {
	pending := make(map[string]int, len(nodes))
	rev := map[string][]string{}
	var ready []string
	for id := range nodes {
		pending[id] = len(deps[id])
		for _, d := range deps[id] {
			rev[d] = append(rev[d], id)
		}
		if pending[id] == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	out := make([]string, 0, len(nodes))
	for len(ready) > 0 {
		cur := ready[0]
		ready = ready[1:]
		out = append(out, cur)
		for _, dn := range rev[cur] {
			pending[dn]--
			if pending[dn] == 0 {
				ready = append(ready, dn)
			}
		}
		sort.Strings(ready)
	}
	return out
}

// ResolveApplicability resolves a node's declared predicate against the
// shipment attributes. A required attribute the shipment does not declare is
// an error (unverifiable), never silently not-applicable.
func ResolveApplicability(n Node, s Shipment) (bool, error) {
	a := n.Applicability
	if a == nil {
		return false, errors.New("applicability is not declared")
	}
	if a.Always {
		return true, nil
	}
	if len(a.Requires) == 0 {
		return false, errors.New("applicability predicate is empty")
	}
	keys := make([]string, 0, len(a.Requires))
	for k := range a.Requires {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	applicable := true
	for _, k := range keys {
		v, ok := s.Attributes[k]
		if !ok {
			return false, fmt.Errorf("shipment attribute %q is not declared", truncate(k))
		}
		match := false
		for _, allowed := range a.Requires[k] {
			if allowed == v {
				match = true
				break
			}
		}
		if !match {
			applicable = false
		}
	}
	return applicable, nil
}

// Evaluate evaluates every node in topological order. Node-level failures are
// reported as node statuses; the returned error is reserved for invalid input.
func (e *Evaluator) Evaluate(in Input) (Report, error) {
	if e == nil {
		return Report{}, fmt.Errorf("%w: nil evaluator", ErrInvalidInput)
	}
	if err := validateShipment(in.Shipment); err != nil {
		return Report{}, err
	}
	if err := e.validateInput(in); err != nil {
		return Report{}, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	rep := Report{ShipmentID: in.Shipment.ID, HeadCommit: in.Shipment.HeadCommit}
	results := make(map[string]NodeResult, len(e.order))
	dup := duplicateDigests(in.Evidence)
	for _, id := range e.order {
		n := e.nodes[id]
		key := e.nodeKey(n, in, results, dup)
		ck := cacheKey(in.Shipment, id)
		if ce, ok := e.cache[ck]; ok && ce.key == key {
			results[id] = ce.result
			rep.CacheHits++
			continue
		}
		res := e.computeNode(n, in, results, dup)
		res.Key = key
		if len(e.cache) >= maxCacheEntries {
			e.cache = map[string]cacheEntry{}
		}
		e.cache[ck] = cacheEntry{key: key, result: res}
		results[id] = res
		rep.Computed++
	}
	rep.Nodes = make([]NodeResult, 0, len(results))
	for _, id := range e.order {
		rep.Nodes = append(rep.Nodes, results[id])
	}
	sort.Slice(rep.Nodes, func(i, j int) bool { return rep.Nodes[i].ID < rep.Nodes[j].ID })
	return rep, nil
}

// maxCacheEntries bounds the per-evaluator cache across many shipments.
const maxCacheEntries = 8 * MaxNodes

func cacheKey(s Shipment, id string) string { return s.ID + "\x00" + s.HeadCommit + "\x00" + id }

// validateInput rejects evidence, waivers and failures naming undeclared nodes
// and failures without a reason. It performs no evaluation and consumes
// nothing, so callers may run it before any irreversible step.
func (e *Evaluator) validateInput(in Input) error {
	if err := validateShipment(in.Shipment); err != nil {
		return err
	}
	for id := range in.Evidence {
		if _, ok := e.nodes[id]; !ok {
			return fmt.Errorf("%w: evidence supplied for undeclared node %q", ErrInvalidInput, truncate(id))
		}
	}
	for id := range in.Waivers {
		if _, ok := e.nodes[id]; !ok {
			return fmt.Errorf("%w: waiver supplied for undeclared node %q", ErrInvalidInput, truncate(id))
		}
	}
	for id, reason := range in.Failures {
		if _, ok := e.nodes[id]; !ok {
			return fmt.Errorf("%w: failure supplied for undeclared node %q", ErrInvalidInput, truncate(id))
		}
		if reason == "" {
			return fmt.Errorf("%w: failure for node %q has no reason", ErrInvalidInput, truncate(id))
		}
		if _, both := in.Evidence[id]; both {
			return fmt.Errorf("%w: node %q has both evidence and an executor failure", ErrInvalidInput, truncate(id))
		}
	}
	return nil
}

func writeField(h hash.Hash, s string) {
	var l [8]byte
	binary.BigEndian.PutUint64(l[:], uint64(len(s)))
	h.Write(l[:])
	h.Write([]byte(s))
}

// nodeKey digests everything that can influence the node's result: the
// declaration, the resolved applicability inputs, the evidence and waiver
// presented, and (transitively, via their keys) every dependency.
func (e *Evaluator) nodeKey(n Node, in Input, results map[string]NodeResult, dup map[string]struct{}) string {
	h := sha256.New()
	writeField(h, "evidencedag/key/v1")
	writeField(h, in.Shipment.ID)
	writeField(h, in.Shipment.HeadCommit)
	nb, err := json.Marshal(n)
	if err != nil {
		return uncacheableKey()
	}
	writeField(h, string(nb))
	if n.Applicability != nil {
		keys := make([]string, 0, len(n.Applicability.Requires))
		for k := range n.Applicability.Requires {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v, ok := in.Shipment.Attributes[k]
			writeField(h, k)
			writeField(h, fmt.Sprintf("%t:%s", ok, v))
		}
	}
	if r, ok := in.Evidence[n.ID]; ok {
		rb, err := json.Marshal(r)
		if err != nil {
			return uncacheableKey()
		}
		writeField(h, "evidence")
		writeField(h, string(rb))
	} else {
		writeField(h, "no-evidence")
	}
	_, isDup := dup[n.ID]
	failure, failed := in.Failures[n.ID]
	writeField(h, fmt.Sprintf("dup:%t|failed:%t", isDup, failed))
	writeField(h, failure)
	if w, ok := in.Waivers[n.ID]; ok {
		writeField(h, fmt.Sprintf("waiver:%s|%s|%s|%s|%s|%s|%s|%t", w.id, w.shipmentID, w.nodeID, w.headCommit, w.principal, w.family, w.nodeDigest, w.valid()))
	} else {
		writeField(h, "no-waiver")
	}
	for _, d := range e.deps[n.ID] {
		writeField(h, d)
		writeField(h, results[d].Key)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// uncacheableKey is a unique key, so a node whose inputs cannot be digested is
// always recomputed and never served from the cache.
func uncacheableKey() string { return "uncacheable:" + randHex(16) }

func (e *Evaluator) computeNode(n Node, in Input, results map[string]NodeResult, dup map[string]struct{}) NodeResult {
	res := NodeResult{ID: n.ID, Family: n.Family, Terminal: n.Terminal, OnUnmet: n.OnUnmet}
	set := func(s Status, reason string) NodeResult {
		res.Status, res.Reason = s, truncate(reason)
		return res
	}

	// Authenticity is established before applicability is consulted.
	rec, has := in.Evidence[n.ID]
	if has {
		if err := authenticate(n, in.Shipment, rec); err != nil {
			return set(StatusError, "evidence rejected: "+err.Error())
		}
		if _, replayed := dup[n.ID]; replayed {
			return set(StatusError, "evidence rejected: digest is presented for more than one node")
		}
	}

	applicable, err := ResolveApplicability(n, in.Shipment)
	if err != nil {
		return set(StatusError, err.Error())
	}
	if !applicable {
		return set(StatusNotApplicable, "applicability predicate excludes this shipment")
	}

	if w, ok := in.Waivers[n.ID]; ok {
		if !w.valid() || !w.boundTo(n) || w.shipmentID != in.Shipment.ID || w.headCommit != in.Shipment.HeadCommit {
			return set(StatusError, "waiver is unverified or not bound to this shipment, node definition and head commit")
		}
		res.Waiver = w.principal
		for _, d := range e.deps[n.ID] {
			if s := results[d].Status; s == StatusUnmet || s == StatusError {
				res.MaskedUpstreams = append(res.MaskedUpstreams, d)
			}
		}
		return set(StatusWaived, "waived by authorized, authenticated principal")
	}

	// A node's own executor failure is accounted for before upstream state is
	// consulted, so it is never reported as merely an unmet upstream.
	if reason, failed := in.Failures[n.ID]; failed && !has {
		return set(StatusError, "executor failed: "+reason)
	}

	// Non-applicable upstream nodes are neutral: they neither block nor
	// satisfy their dependents. Unmet upstream nodes propagate their strongest
	// blocking semantics, so a report-mode terminal cannot launder a blocking
	// upstream into an advisory.
	var unmetUp, errUp string
	blockUp := false
	for _, d := range e.deps[n.ID] {
		switch results[d].Status {
		case StatusError:
			if errUp == "" {
				errUp = d
			}
		case StatusUnmet:
			if unmetUp == "" {
				unmetUp = d
			}
			if results[d].OnUnmet == OnUnmetBlock {
				blockUp = true
			}
		}
	}
	if errUp != "" {
		return set(StatusError, "upstream node "+errUp+" is unverifiable")
	}
	if unmetUp != "" {
		if blockUp {
			res.OnUnmet = OnUnmetBlock
		}
		return set(StatusUnmet, "upstream node "+unmetUp+" is unmet")
	}

	return verifyEvidence(n, rec, has, set)
}

// verifyEvidence applies the S4 U4 conformance rule: a node is verified only
// by conformant evidence of the bound family whose envelope status is pass.
func verifyEvidence(n Node, rec EvidenceRecord, has bool, set func(Status, string) NodeResult) NodeResult {
	want, ok := evidenceFamilyFor(n.Family)
	if !ok {
		return set(StatusError, "no S4 evidence family is registered for family "+string(n.Family))
	}
	if !has {
		return set(StatusUnmet, "no evidence supplied")
	}
	if len(rec.Artifact) == 0 {
		return set(StatusError, "evidence record carries no artifact")
	}
	a, _, err := faultline.DecodeAndValidate(rec.Artifact)
	if err != nil {
		return set(StatusError, "evidence does not conform to the S4 contract: "+faultline.SanitizeDiagnostic(err.Error()))
	}
	if a.NodeFamily != want {
		return set(StatusError, fmt.Sprintf("evidence family %q does not match required %q", a.NodeFamily, want))
	}
	if a.Status != faultline.StatusPass {
		return set(StatusUnmet, "evidence status is "+a.Status)
	}
	return set(StatusVerified, "conformant passing evidence")
}

// Node returns the result for a node ID.
func (r Report) Node(id string) (NodeResult, bool) {
	i := sort.Search(len(r.Nodes), func(i int) bool { return r.Nodes[i].ID >= id })
	if i < len(r.Nodes) && r.Nodes[i].ID == id {
		return r.Nodes[i], true
	}
	return NodeResult{}, false
}

// Rollup computes the shipment roll-up over applicable terminal nodes. An
// empty applicable-terminal set is unverifiable, never "all satisfied".
func (r Report) Rollup() Rollup {
	ru := Rollup{Unmet: []string{}, Unverifiable: []string{}, Advisory: []string{}}
	for _, n := range r.Nodes {
		if !n.Terminal || n.Status == StatusNotApplicable {
			continue
		}
		ru.ApplicableTerminals++
		switch n.Status {
		case StatusVerified, StatusWaived:
			ru.ProvenTerminals++
		case StatusUnmet:
			if n.OnUnmet == OnUnmetBlock {
				ru.Unmet = append(ru.Unmet, n.ID)
			} else {
				ru.Advisory = append(ru.Advisory, n.ID)
			}
		default:
			ru.Unverifiable = append(ru.Unverifiable, n.ID)
		}
	}
	switch {
	case ru.ApplicableTerminals == 0:
		ru.State, ru.Reason = RollupUnverifiable, "no applicable terminal node: nothing can be verified"
	case len(ru.Unverifiable) > 0:
		ru.State, ru.Reason = RollupUnverifiable, "an applicable terminal node is unverifiable"
	case len(ru.Unmet) > 0:
		ru.State, ru.Reason = RollupBlocked, "an applicable terminal node is unmet"
	case ru.ProvenTerminals == 0:
		ru.State, ru.Reason = RollupUnproven, "no applicable terminal node is verified or waived: advisory nodes alone prove nothing"
	default:
		ru.State, ru.Reason = RollupReady, "every applicable terminal node is verified or waived, or advisory"
	}
	return ru
}
