// Package main provides the wt binary entry point.
package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	environ := os.Environ()
	env := make(map[string]string, len(environ))

	for _, e := range environ {
		if k, v, ok := strings.Cut(e, "="); ok {
			env[k] = v
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	exitCode := Run(os.Stdin, os.Stdout, os.Stderr, os.Args, env, sigCh)
	os.Exit(exitCode)
}
