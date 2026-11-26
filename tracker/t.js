// tally tracker. Hand-written ES2017, minified into t.min.js by `go generate ./tracker`.
//
// The server serves it at /s/{site}.js and replaces the two placeholder strings below: the
// site ID, and "1" when the site's domain is localhost (so local testing is tracked).
//
// It sets no cookies and reads nothing but the page URL, the referrer and the screen width.
// localStorage is only used for the owner's opt-out flag (?tally=ignore / ?tally=track).
(function (site, allowLocal) {
  var d = document,
    w = window,
    l = location,
    n = navigator,
    h = history,
    script = d.currentScript;
  if (!script) return;

  var endpoint = new URL(script.src).origin + "/e";

  try {
    if (/[?&]tally=ignore/.test(l.search)) localStorage.tally_ignore = 1;
    if (/[?&]tally=track/.test(l.search)) localStorage.removeItem("tally_ignore");
    if (localStorage.tally_ignore) return;
  } catch (e) {
    // Storage blocked: track as usual.
  }

  if (n.webdriver || w._phantom || w.__nightmare || l.protocol == "file:") return;
  if (allowLocal != 1 && /^(localhost|127\.|\[::1\]$|0\.0\.0\.0$)|\.local$/.test(l.hostname)) return;

  var send = (p) => {
    p.s = site;
    p = JSON.stringify(p);
    // A string body is sent as text/plain, so the browser makes no CORS preflight.
    n.sendBeacon ? n.sendBeacon(endpoint, p) : fetch(endpoint, { method: "POST", body: p, keepalive: true });
  };

  // The page URL without its hash, keeping only utm_* and ref from the query string.
  var pageURL = () => {
    var keep = new URLSearchParams();
    new URLSearchParams(l.search).forEach((v, k) => /^(utm_\w+|ref)$/.test(k) && keep.append(k, v));
    keep += "";
    return l.origin + l.pathname + (keep ? "?" + keep : "");
  };

  // Engaged time: how long the page was visible. It is sent when the page is hidden or left,
  // and before an SPA navigation; each ping carries only the time since the previous one.
  var engaged = 0,
    since = 0,
    current,
    lastPath,
    referrer = d.referrer;

  var flush = () => {
    if (since) {
      engaged += Date.now() - since;
      since = Date.now();
    }
    if (current && engaged > 0) send({ n: "engagement", u: current, e: engaged });
    engaged = 0;
  };

  // Both pagehide and visibilitychange fire when a tab closes; whichever comes second finds the
  // clock stopped and sends nothing.
  var hide = () => {
    flush();
    since = 0;
  };

  var visibility = () => {
    if (d.visibilityState == "hidden") hide();
    else if (!since) since = Date.now();
  };

  var page = () => {
    if (lastPath == l.pathname) return;
    flush();
    lastPath = l.pathname;
    current = pageURL();
    send({ n: "pageview", u: current, r: referrer, w: screen.width });
    // Later SPA pageviews have no outside referrer of their own.
    referrer = "";
  };

  ["pushState", "replaceState"].forEach((k) => {
    var original = h[k];
    h[k] = function () {
      var r = original.apply(this, arguments);
      page();
      return r;
    };
  });
  w.addEventListener("popstate", page);
  d.addEventListener("visibilitychange", visibility);
  w.addEventListener("pagehide", hide);

  var start = () => {
    visibility();
    page();
  };
  // A prerendered page is only counted once the visitor actually opens it.
  d.prerendering ? d.addEventListener("prerenderingchange", start, { once: true }) : start();
})("__TALLY_SITE__", "__TALLY_LOCAL__");
