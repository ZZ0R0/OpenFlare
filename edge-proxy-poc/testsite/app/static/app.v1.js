// OpenFlare TestSite — app.v1.js
// Static cacheable JavaScript asset for proxy cache testing.
(function() {
    "use strict";
    console.log("OpenFlare TestSite loaded — version 1");
    document.addEventListener("DOMContentLoaded", function() {
        var el = document.getElementById("app-status");
        if (el) { el.textContent = "App loaded"; }
    });
})();
