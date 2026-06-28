package core

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGoSourceFilesDoNotHaveUTF8BOM(t *testing.T) {
	root := filepath.Join("..", "..")
	bom := []byte{0xEF, 0xBB, 0xBF}

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.HasPrefix(data, bom) {
			t.Fatalf("%s has UTF-8 BOM", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
