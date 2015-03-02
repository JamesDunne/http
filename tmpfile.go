package main

import (
	"fmt"
	"os"
	"path"
	"time"
)

func HomeDir() string {
	return os.Getenv("HOME")
}

func ConfigDir() string {
	p := path.Join(HomeDir(), ".config/http")
	os.MkdirAll(p, 0700)
	return p
}

func SessionID() (sessionID string) {
	sessionID = os.Getenv("HTTPCLI_SESSION_ID")
	if sessionID != "" {
		return
	}

	datestamp := time.Now().Format("2006-01-02")

	//	// OS X Terminal exports this env var; seems convenient enough:
	//	sessionID = os.Getenv("TERM_SESSION_ID")
	//	if sessionID != "" {
	//		return
	//	}

	// `os.Getppid()` is a nice choice for user-interactive shells but is bad for scripts
	// since sub-shell processes are spawned to exec scripts normally, unless `source`d.
	// Scripts should specify explicit session ids via HTTPCLI_SESSION_ID overriding env var.
	sessionID = fmt.Sprintf("%s-%08x", datestamp, os.Getppid())
	return
}

var _env_path string

func env_path() string {
	if _env_path == "" {
		_env_path = path.Join(ConfigDir(), path.Clean(SessionID())+".env")
	}
	return _env_path
}
