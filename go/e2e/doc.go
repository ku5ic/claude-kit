// Package e2e holds end-to-end tests: each builds nothing of its own, runs
// the kit binary the way hooks.json and the bin/kit launcher do (argv, stdin, cwd,
// HOME, CLAUDE_PLUGIN_ROOT), and checks exit status and output.
package e2e
