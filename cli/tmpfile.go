// +build !darwin

package main

import "os"

func TempDir() string {
	return os.TempDir()
}
