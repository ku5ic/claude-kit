package bashguard

import "github.com/ku5ic/claude-kit/go/internal/hook"

// Step is one of bash-dispatch's guards, by the name it reports under.
type Step struct {
	Name  string
	Check hook.Check
}

// Steps are bash-dispatch's guards in the order it runs them. kit hook
// runs them and kit explain bash prints each one's verdict, so the two
// can't disagree on a command.
var Steps = []Step{
	{"guard-bash", Check},
	{"guard-attribution", CheckAttribution},
	{"guard-commit", CheckCommit},
	{"guard-pr", CheckPR},
}
