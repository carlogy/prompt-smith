package server

import (
	"fmt"
	"os/exec"
	"runtime"
)

// browserCommand returns the name and args of the OS command used to
// open url in the user's default browser, for the given GOOS value.
// Pure and unit-tested on its own; openBrowser (the only caller) is
// untested by design - it starts a real process.
func browserCommand(goos, url string) (name string, args []string, err error) {
	switch goos {
	case "darwin":
		return "open", []string{url}, nil
	case "windows":
		// rundll32 is the conventional way to invoke the shell's URL
		// handler directly, without going through cmd.exe's "start" (a
		// shell builtin, not a standalone executable, with its own
		// quoting quirks around the first argument).
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}, nil
	case "linux", "freebsd", "openbsd", "netbsd":
		return "xdg-open", []string{url}, nil
	default:
		return "", nil, fmt.Errorf("opening a browser is not supported on %s", goos)
	}
}

// openBrowser best-effort opens url in the user's default browser.
// Never fatal to the caller: if the command can't be found or fails
// to start (no desktop environment, headless CI, an unsupported OS),
// the URL is already printed and can be opened manually.
func openBrowser(url string) error {
	name, args, err := browserCommand(runtime.GOOS, url)
	if err != nil {
		return err
	}
	// #nosec G204 -- name/args come from browserCommand's fixed, tested
	// switch over runtime.GOOS (never external input); url is this
	// server's own loopback address, built from listener.Addr() in
	// Serve, never user- or attacker-controlled. exec.Command also
	// execs the named binary directly via argv - it never invokes a
	// shell - so no metacharacter injection is possible here regardless
	// of url's content. G204 flags any non-literal exec.Command
	// argument unconditionally (confirmed empirically: it still flags
	// even after explicit regex-validating url first), so this is the
	// intended resolution for a subprocess call that legitimately needs
	// a dynamic argument, not a workaround for a real risk.
	cmd := exec.Command(name, args...)
	return cmd.Start()
}
