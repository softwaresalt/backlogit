package policyengine

// maxOverrideTokens caps the override tokens in one request.
const maxOverrideTokens = 8

// prepareOverrides returns the tokens that may be handed to the core gate and
// the denials for the rest. Until override authentication is wired every
// token is denied: nothing is ever waived implicitly.
func (e *Engine) prepareOverrides(req Request) ([][]byte, []OverrideDenial) {
	var denials []OverrideDenial
	for i := range req.OverrideTokens {
		denials = append(denials, OverrideDenial{Index: i, Reason: "overrides are not enabled"})
	}
	return nil, denials
}
