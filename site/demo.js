// The demo page. It loads ascc as WebAssembly from a script the build embedded (ascc.wasm.js), so
// nothing is fetched, then replays the planted tool calls through the same hook the CLI runs.
// Every decision, audit line and status change comes from the Go code; this file only draws them.
(function () {
  'use strict';
  var F = globalThis.ASCC_FIXTURES;
  var $ = function (id) { return document.getElementById(id); };
  var esc = function (s) {
    return String(s).replace(/[&<>"']/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]; });
  };

  var ascc = null;           // set once the WebAssembly program is running
  var registry = F.registry; // JSON text; the kill switch rewrites it
  var answers = [];          // the last answer for each planted call
  var running = '';          // the status line when all is well
  var policyText = function () { return Object.keys(F.policies).sort().map(function (k) { return F.policies[k]; }).join('\n'); };

  function agents() {
    try { return JSON.parse(registry).agents || []; } catch (e) { return []; }
  }

  function replay(i) {
    var out = JSON.parse(ascc.hook(registry, JSON.stringify({ 'policy.cedar': $('policy').value }), F.calls[i].payload, $('agent').value, $('log').value));
    answers[i] = out;
    if (typeof out.log === 'string') $('log').value = out.log;
  }

  function replayAll() {
    F.calls.forEach(function (_, i) { replay(i); });
    draw();
  }

  // guard runs one visitor action. If the engine throws, every earlier answer is dropped, so a stale
  // allow never stays on screen, and the page says the engine stopped.
  function guard(action) {
    return function (e) {
      try {
        if (action(e) !== false) $('status').textContent = running;
      } catch (err) {
        answers = [];
        try { draw(); } catch (again) { $('calls').innerHTML = ''; }
        $('status').textContent = 'The policy engine stopped, so nothing above is an answer: ' + (err && err.message ? err.message : err) + '. Reload the page.';
      }
    };
  }

  function draw() {
    $('calls').innerHTML = F.calls.map(function (call, i) {
      var p = {};
      try { p = JSON.parse(call.payload); } catch (e) {}
      var input = p.tool_input || {};
      var a = answers[i];
      var verdict = a ? (a.decision === 'allow' ? 'allow' : 'deny') : '';
      return '<article class="call' + (verdict ? ' is-' + verdict : '') + '">' +
        '<span class="badge' + (verdict ? ' is-' + verdict : '') + '">' + (verdict || 'not run') + '</span>' +
        '<div class="what"><div class="cmd">' + esc(input.command || input.file_path || call.name) + '</div>' +
        '<div class="meta">' + esc(p.tool_name || '') + (input.description ? ' · “' + esc(input.description) + '”' : '') + '</div></div>' +
        '<button class="btn" type="button" data-i="' + i + '">Replay</button>' +
        '<div class="reason">' + (a ? esc(a.reason || '') : '') + '</div>' +
        '</article>';
    }).join('');

    $('agents').innerHTML = agents().map(function (a) {
      var stopped = a.status !== 'active';
      var button = a.status === 'retired' ? '' :
        '<button class="btn" type="button" data-agent="' + esc(a.id) + '" data-event="' + (stopped ? 'revive' : 'kill') + '">' + (stopped ? 'Revive' : 'Kill') + '</button>';
      return '<div class="agent"><span class="badge ' + (stopped ? 'is-deny' : 'is-allow') + '">' + esc(a.status) + '</span>' +
        '<span class="name">' + esc(a.id) + '</span>' +
        '<span class="meta">owner ' + esc(a.owner || 'none') + ' · risk tier ' + esc(a.risk_tier) + '</span>' + button + '</div>';
    }).join('');

    verify();
  }

  function verify() {
    var v = JSON.parse(ascc.verify($('log').value));
    $('verify').textContent = v.report;
    $('verify').className = 'verify ' + (v.ok ? 'is-ok' : 'is-bad');
  }

  function wire() {
    $('calls').addEventListener('click', guard(function (e) {
      var b = e.target.closest('button[data-i]');
      if (!b) return false;
      replay(Number(b.dataset.i));
      draw();
    }));
    $('replayAll').addEventListener('click', guard(replayAll));
    $('agent').addEventListener('change', guard(replayAll));
    $('policyReset').addEventListener('click', guard(function () { $('policy').value = policyText(); replayAll(); }));
    $('log').addEventListener('input', guard(function () { verify(); return false; }));
    $('logReset').addEventListener('click', guard(function () { $('log').value = ''; replayAll(); }));
    $('agents').addEventListener('click', guard(function (e) {
      var b = e.target.closest('button[data-agent]');
      if (!b) return false;
      var out = JSON.parse(ascc.setStatus(registry, b.dataset.agent, b.dataset.event, $('log').value));
      // As in the CLI, the registry changes even when the event couldn't be recorded.
      if (typeof out.registry === 'string') registry = out.registry;
      if (typeof out.log === 'string') $('log').value = out.log;
      replayAll();
      if (out.error) {
        $('status').textContent = 'ascc: ' + out.error;
        return false;
      }
    }));
  }

  async function boot() {
    $('policy').value = policyText();
    $('agent').innerHTML = agents().map(function (a) { return '<option>' + esc(a.id) + '</option>'; }).join('');
    $('agent').value = 'dev-agent';

    var raw = atob(globalThis.ASCC_WASM);
    var bytes = new Uint8Array(raw.length);
    for (var i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
    var go = new globalThis.Go();
    var result = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(result.instance); // returns when the program exits, which it never does
    ascc = globalThis.ascc;
    if (!ascc) throw new Error('the WebAssembly program did not start');

    wire();
    replayAll();
    $('version').textContent = 'ascc ' + ascc.version;
    running = 'Running ascc ' + ascc.version + ' as WebAssembly. No request has left this page, and none will.';
    $('status').textContent = running;
  }

  // Anything that stops the engine loading is shown, never drawn as an allow.
  globalThis.asccReady = boot().catch(function (err) {
    $('status').textContent = 'The demo could not start: ' + (err && err.message ? err.message : err);
    throw err;
  });
})();
