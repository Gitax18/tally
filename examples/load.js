// Loads the tracker the way a real site's <script> tag would, but takes the site ID and the
// server origin from the query string (remembered for the tab), so one demo works with any site:
//
//   http://localhost:8099/demo.html?site=<site id>&api=http://127.0.0.1:8090
//
// A real site just pastes the snippet:
//   <script defer src="https://a.yourdomain.com/s/<site id>.js"></script>
(function () {
  var q = new URLSearchParams(location.search);
  var site = q.get("site") || sessionStorage.getItem("tally_demo_site");
  var api = q.get("api") || sessionStorage.getItem("tally_demo_api") || "http://127.0.0.1:8090";
  var status = document.getElementById("status");
  if (!site) {
    status.textContent = "Add ?site=<site id> to the URL (create a localhost site first).";
    return;
  }
  sessionStorage.setItem("tally_demo_site", site);
  sessionStorage.setItem("tally_demo_api", api);
  var s = document.createElement("script");
  s.defer = true;
  s.src = api + "/s/" + site + ".js";
  s.onload = function () {
    status.textContent = "Tracking with site " + site + " via " + api;
  };
  s.onerror = function () {
    status.textContent = "Could not load " + s.src;
  };
  document.head.appendChild(s);
})();
