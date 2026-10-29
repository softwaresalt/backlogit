package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestAttestationEnvelopeSourceShape is the source-shape harness for the
// signed delivery-evidence attestation envelope declaration (169.001-T). It
// inspects the SOURCE of attestation_envelope.go via go/parser/go/ast rather
// than referencing the declared symbols, so it compiles before the
// declaration exists and fails on an assertion, never a build error. The
// declaration owns the exact JSON tag on every field; the parser (169.010-T)
// must not mutate this shape.
func TestAttestationEnvelopeSourceShape(t *testing.T) {
	want := map[string][][2]string{
		"AttestationEnvelope": {
			{"PayloadType", "`json:\"payloadType\"`"},
			{"Payload", "`json:\"payload\"`"},
			{"Signatures", "`json:\"signatures\"`"},
		},
		"AttestationSignature": {
			{"KeyID", "`json:\"keyid\"`"},
			{"Sig", "`json:\"sig\"`"},
			{"Algo", "`json:\"sig_algo\"`"},
		},
		"AttestationStatement": {
			{"Type", "`json:\"_type\"`"},
			{"Subject", "`json:\"subject\"`"},
			{"PredicateType", "`json:\"predicateType\"`"},
			{"Predicate", "`json:\"predicate\"`"},
		},
		"AttestationSubject": {
			{"Name", "`json:\"name\"`"},
			{"Digest", "`json:\"digest\"`"},
		},
		"AttestationPredicate": {
			{"ShipmentID", "`json:\"shipment_id\"`"},
			{"MergeSHA", "`json:\"merge_sha\"`"},
			{"ManifestDigest", "`json:\"manifest_digest\"`"},
			{"ClosureContentHash", "`json:\"closure_content_hash\"`"},
			{"SignerIdentity", "`json:\"signer_identity\"`"},
			{"IssuedAt", "`json:\"issued_at\"`"},
			{"ExpiresAt", "`json:\"expires_at\"`"},
		},
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "attestation_envelope.go", nil, parser.AllErrors)
	if err != nil {
		t.Errorf("parse attestation_envelope.go: %v", err)
		return
	}

	structs := map[string]*ast.StructType{}
	ast.Inspect(file, func(n ast.Node) bool {
		if spec, ok := n.(*ast.TypeSpec); ok {
			if st, ok := spec.Type.(*ast.StructType); ok {
				structs[spec.Name.Name] = st
			}
		}
		return true
	})

	for typeName, fields := range want {
		st, ok := structs[typeName]
		if !ok {
			t.Errorf("expected struct type %s in attestation_envelope.go", typeName)
			continue
		}
		tags := map[string]string{}
		for _, f := range st.Fields.List {
			tag := ""
			if f.Tag != nil {
				tag = f.Tag.Value
			}
			for _, name := range f.Names {
				tags[name.Name] = tag
			}
		}
		for _, field := range fields {
			got, ok := tags[field[0]]
			if !ok {
				t.Errorf("%s: expected field %s", typeName, field[0])
				continue
			}
			if got != field[1] {
				t.Errorf("%s.%s: json tag = %s, want %s", typeName, field[0], got, field[1])
			}
		}
	}
}
