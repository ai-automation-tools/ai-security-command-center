// Command ascc is the AI Security Command Center CLI. For now it only prints its version;
// the policy decision point, the Claude Code hook and the audit log arrive one roadmap item at a time.
package main

import (
	"fmt"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/version"
)

func main() {
	fmt.Println("ascc " + version.Version)
}
