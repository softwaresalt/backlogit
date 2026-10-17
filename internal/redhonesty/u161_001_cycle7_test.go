package redhonesty

import "testing"

// TestU161_001_IdentifyNewTests_EmptyResultListIsStillATest asserts that a test
// declared with an empty (but non-nil) result list, `func TestX(t *testing.T) ()`,
// is discovered exactly like `func TestX(t *testing.T)`, while a non-empty
// result list is not a test.
func TestU161_001_IdentifyNewTests_EmptyResultListIsStillATest(t *testing.T) {
	src := "package p\nimport \"testing\"\n" +
		"func TestNoResults(t *testing.T) {}\n" +
		"func TestEmptyResults(t *testing.T) () {}\n" +
		"func TestOneResult(t *testing.T) int { return 0 }\n" +
		"func TestNamedResult(t *testing.T) (err error) { return }\n"
	got, err := IdentifyNewTests([]FileChange{{Path: "p/x_test.go", Status: 'A', Head: []byte(src)}})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, n := range got {
		names = append(names, n.Name)
	}
	want := []string{"TestEmptyResults", "TestNoResults"}
	if len(names) != len(want) || names[0] != want[0] || names[1] != want[1] {
		t.Fatalf("discovered %v, want %v", names, want)
	}
}
