package main

import (
	"path"
	"runtime"
)

var baseDir string = func() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("No caller information")
	}
	return path.Dir(filename)
}()

// secretsPath returns the path to a file in the secrets directory
func secretsPath(filename string) string {
	return path.Join(baseDir, "secrets", filename)
}
