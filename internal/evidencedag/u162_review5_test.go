package evidencedag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU162_Review5_TrustRejectsConflictingKeyAliases(t *testing.T) {
	a := newSigner("k-a")
	cases := map[string][]map[string]any{
		"same key automation and human": {a.keyJSON("alice", "human"), func() map[string]any {
			m := a.keyJSON("alice", "automation")
			m["key_id"] = "k-a-bot"
			return m
		}()},
		"same key different principals": {a.keyJSON("alice", "human"), func() map[string]any {
			m := a.keyJSON("mallory", "human")
			m["key_id"] = "k-a-2"
			return m
		}()},
		"same key same principal and kind": {a.keyJSON("alice", "human"), func() map[string]any {
			m := a.keyJSON("alice", "human")
			m["key_id"] = "k-a-3"
			return m
		}()},
		"key_id case alias": {a.keyJSON("alice", "human"), func() map[string]any {
			b := newSigner("other")
			m := b.keyJSON("bob", "human")
			m["key_id"] = "K-A"
			return m
		}()},
		"principal case alias":  {a.keyJSON("alice", "human"), newSigner("o2").keyJSON("Alice", "human")},
		"principal mixed kinds": {a.keyJSON("alice", "human"), newSigner("o3").keyJSON("alice", "automation")},
	}
	for name, keys := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseTrust(trustDoc(keys, map[string][]string{}))
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrTrustInvalid)
		})
	}
}

func TestU162_Review5_WaiverBoundToNodeDefinition(t *testing.T) {
	f := newFixture(t)
	d := decl([]Node{pnode("t", true)})
	w, err := f.svc.Accept(signClaims(f.alice, f.claims()), ship(), lookupFor(d))
	require.NoError(t, err)

	eval := func(n Node) NodeResult {
		e := mustEval(t, decl([]Node{n}))
		r, err := e.Evaluate(Input{Shipment: ship(), Waivers: map[string]Waiver{"t": w}})
		require.NoError(t, err)
		nr, ok := r.Node("t")
		require.True(t, ok)
		return nr
	}
	assert.Equal(t, StatusWaived, eval(pnode("t", true)).Status)

	other := pnode("t", true)
	other.Family = FamilyRedTestHonesty
	assert.Equal(t, StatusError, eval(other).Status, "different family must not honor the waiver")

	altered := pnode("t", true)
	altered.ProducingTask = testTaskB
	assert.Equal(t, StatusError, eval(altered).Status, "altered definition must not honor the waiver")

	altered = pnode("t", true)
	altered.Applicability = &Applicability{Requires: map[string][]string{"surface": {"cli"}}}
	assert.Equal(t, StatusError, eval(altered).Status)
}
