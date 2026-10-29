package core

// AttestationEnvelope is the DSSE-style signed envelope a CI or release
// system emits over the reconcile delivery evidence (169.001-T). Payload is
// the base64-encoded in-toto statement (AttestationStatement); the signature
// is computed over the decoded payload bytes. This is a declaration only: the
// parser is 169.010-T and verification is 169.002-T. The JSON tag on every
// field is owned by this declaration.
type AttestationEnvelope struct {
	PayloadType string                 `json:"payloadType"`
	Payload     string                 `json:"payload"`
	Signatures  []AttestationSignature `json:"signatures"`
}

// AttestationSignature is one signature entry of an AttestationEnvelope. Algo
// is the payload-declared algorithm; it is never trusted and must equal the
// pinned algorithm of the resolved trust anchor.
type AttestationSignature struct {
	KeyID string `json:"keyid"`
	Sig   string `json:"sig"`
	Algo  string `json:"sig_algo"`
}

// AttestationStatement is the in-toto statement carried in the envelope
// payload.
type AttestationStatement struct {
	Type          string               `json:"_type"`
	Subject       []AttestationSubject `json:"subject"`
	PredicateType string               `json:"predicateType"`
	Predicate     AttestationPredicate `json:"predicate"`
}

// AttestationSubject names one attested subject and its digests.
type AttestationSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// AttestationPredicate is the canonical delivery-evidence statement: the
// shipment id, merge SHA, manifest digest and closure content hash that the
// reconcile request computes, plus the signer identity and validity window.
type AttestationPredicate struct {
	ShipmentID         string `json:"shipment_id"`
	MergeSHA           string `json:"merge_sha"`
	ManifestDigest     string `json:"manifest_digest"`
	ClosureContentHash string `json:"closure_content_hash"`
	SignerIdentity     string `json:"signer_identity"`
	IssuedAt           string `json:"issued_at"`
	ExpiresAt          string `json:"expires_at"`
}
