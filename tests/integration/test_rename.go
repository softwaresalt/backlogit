package main
import (
	"fmt"
	"os"
	"path/filepath"
)
func main() {
	dir, _ := os.MkdirTemp("", "test")
	defer os.RemoveAll(dir)
	f1 := filepath.Join(dir, "f1.txt")
	f2 := filepath.Join(dir, "f2.txt")
	os.WriteFile(f1, []byte("1"), 0644)
	os.WriteFile(f2, []byte("2"), 0644)
	err := os.Rename(f1, f2)
	fmt.Printf("Rename error: %v\n", err)
	data, _ := os.ReadFile(f2)
	fmt.Printf("F2 content: %s\n", string(data))
}
