package main

// NOTE(jsd): OS X returns some unwritable temporary directory from $TMPDIR. Using /tmp instead.
func TempDir() string {
	return "/tmp"
}
