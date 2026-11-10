//go:build js && wasm

// Command ascc-wasm is the browser demo's build of ascc: src/internal/demo exposed to the page as
// the global `ascc`. Every function takes and returns strings (JSON where it says so) and none of
// them can reach a file or the network: this program links neither.
package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
	"time"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/demo"
	"github.com/ai-automation-tools/ai-security-command-center/src/internal/version"
)

func main() {
	js.Global().Set("ascc", js.ValueOf(map[string]any{
		"version": version.Version,
		// hook(registry, policiesJSON, payload, agent, log) -> JSON demo.Answer
		"hook": fn(5, func(a []string) any {
			var policies map[string]string
			if err := json.Unmarshal([]byte(a[1]), &policies); err != nil {
				policies = nil // no policy loads, so the answer is a deny that says so
			}
			return demo.Hook(a[0], policies, a[2], a[3], a[4], now())
		}),
		// verify(log) -> JSON {ok, report}
		"verify": fn(1, func(a []string) any {
			ok, report := demo.Verify(a[0])
			return map[string]any{"ok": ok, "report": report}
		}),
		// setStatus(registry, agent, "kill"|"revive", log) -> JSON {registry, log, error}
		"setStatus": fn(4, func(a []string) any {
			reg, log, err := demo.SetStatus(a[0], a[1], a[2], a[3], now())
			out := map[string]any{"registry": reg, "log": log, "error": ""}
			if err != nil {
				out["error"] = err.Error()
			}
			return out
		}),
	}))
	select {} // stay alive for the page's calls
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// fn wraps f as a JS function of n string arguments returning JSON. A panic or a wrong call comes
// back as a deny-shaped error, never as an allow or an uncaught exception.
func fn(n int, f func(args []string) any) js.Func {
	return js.FuncOf(func(_ js.Value, args []js.Value) (out any) {
		fail := func(msg string) string {
			b, _ := json.Marshal(map[string]any{"decision": "deny", "ok": false, "error": msg, "reason": msg, "report": msg})
			return string(b)
		}
		defer func() {
			if r := recover(); r != nil {
				out = fail(fmt.Sprintf("internal error: %v", r))
			}
		}()
		if len(args) != n {
			return fail(fmt.Sprintf("expected %d arguments", n))
		}
		in := make([]string, n)
		for i, a := range args {
			if a.Type() != js.TypeString {
				return fail("every argument must be a string")
			}
			in[i] = a.String()
		}
		b, err := json.Marshal(f(in))
		if err != nil {
			return fail(err.Error())
		}
		return string(b)
	})
}
