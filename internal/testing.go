package internal

import (
	"os"
	"path"
	"testing"
)

func fixturePath(name string) string {
	return path.Join("fixtures", name)
}

func fixtureExists(name string) bool {
	f, err := os.Open(fixturePath(name))
	if err != nil {
		return false
	}
	defer f.Close()

	return true
}

func fixtureContent(name string) []byte {
	result, _ := os.ReadFile(fixturePath(name))
	return result
}

func fixtureLength(name string) int64 {
	info, _ := os.Stat(fixturePath(name))
	return info.Size()
}

func usingEnvVar(t *testing.T, key, value string) {
	old, found := os.LookupEnv(key)
	os.Setenv(key, value)

	t.Cleanup(func() {
		if found {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

func usingProgramArgs(t *testing.T, args ...string) {
	old := os.Args
	os.Args = args

	t.Cleanup(func() {
		os.Args = old
	})
}
