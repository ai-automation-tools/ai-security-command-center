# Browser page test

The demo page, driven in headless Chromium by [Playwright](https://playwright.dev), with the network taken
away. `render.js` one folder up walks the same steps against a stand-in for the DOM; this one uses the
browser's own DOM, content policy and WebAssembly engine.

Every request the page makes is stopped before it leaves the browser. One for a file of the built site is
answered from disk; anything else is refused and fails the test, and so does any console error, which is how
a content-policy violation shows.

## Run it

From this folder, once:

```bash
npm ci
npx playwright install chromium
```

Then from the repository root, in bash:

```bash
ASCC_BROWSER_TEST=1 go test -count=1 -run TestBuiltPageInABrowser ./src/cmd/buildsite
```

In PowerShell, set `$env:ASCC_BROWSER_TEST = "1"` first. Without the variable the test skips. To use a
Chromium you already have and skip the download, put the path to its executable in `ASCC_CHROMIUM`.

`playwright` is the one package here, pinned exactly in `package.json` and by hash in `package-lock.json`. It
is a test tool: nothing in this folder is copied into the site.
