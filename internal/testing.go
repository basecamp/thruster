package internal

import (
	"os"
	"path"
)

func fixturePath(name string) string {
	return path.Join("fixtures", name)
}

func fixtureContent(name string) []byte {
	result, _ := os.ReadFile(fixturePath(name))
	return result
}

func fixtureLength(name string) int64 {
	info, _ := os.Stat(fixturePath(name))
	return info.Size()
}
