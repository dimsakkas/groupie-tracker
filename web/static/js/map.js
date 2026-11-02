// map.js - Leaflet loader + the tour map on the artist page.
//
// Leaflet comes from the unpkg CDN and is loaded only when a map is needed
// (artist page, or when the user opens the world map on the home page).
// The artist page asks /api/artist/{id}/geo. While the server is still
// geocoding ("ready": false) we keep asking every few seconds.
(function () {
  'use strict';

  var LEAFLET = 'https://unpkg.com/leaflet@1.9.4/dist/';
  var CSS_HASH = 'sha256-p4NxAoJBhIIN+hmNHrzRCf9tD/miZyoHS5obTRR9BMY=';
  var JS_HASH = 'sha256-20nQCchB9co0qIjJZRGuk2/Z9VM+kNiyxNV1lvTlZBo=';
  var POLL_MS = 3000;
  var MAX_POLLS = 200; // about ten minutes, then we stop asking

  window.GT = window.GT || {};

  // ---- Leaflet loader (shared with worldmap.js) ----

  var leafletPromise = null;

  function loadLeaflet() {
    if (window.L) return Promise.resolve(window.L);
    if (leafletPromise) return leafletPromise;

    var css = new Promise(function (resolve, reject) {
      var link = document.createElement('link');
      link.rel = 'stylesheet';
      link.href = LEAFLET + 'leaflet.css';
      link.integrity = CSS_HASH;
      link.crossOrigin = '';
      link.onload = resolve;
      link.onerror = function () { reject(new Error('Leaflet CSS failed to load')); };
      document.head.appendChild(link);
    });
    var js = new Promise(function (resolve, reject) {
      var script = document.createElement('script');
      script.src = LEAFLET + 'leaflet.js';
      script.integrity = JS_HASH;
      script.crossOrigin = '';
      script.onload = resolve;
      script.onerror = function () { reject(new Error('Leaflet failed to load')); };
      document.head.appendChild(script);
    });

    leafletPromise = Promise.all([css, js]).then(function () { return window.L; }).catch(function (err) {
      leafletPromise = null; // allow another try
      throw err;
    });
    return leafletPromise;
  }

  function addTiles(L, map) {
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
    }).addTo(map);
  }

  window.GT.loadLeaflet = loadLeaflet;
  window.GT.addTiles = addTiles;

  // ---- artist page ----

  var article = document.querySelector('.artist[data-artist-id]');
  var mapEl = document.getElementById('map');
  if (!article || !mapEl) return;

  var artistId = article.dataset.artistId;
  var statusEl = document.getElementById('map-status');
  var distanceEl = document.getElementById('distance');
  var playBtn = document.getElementById('play');
  var playInfo = document.getElementById('play-info');
  var listedLocations = document.querySelectorAll('.concerts > li').length;
  var pretty = function (raw) { return (window.GT.prettyLocation || String)(raw); };
  var reduceMotion = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  function numberIcon(L, n) {
    return L.divIcon({
      className: 'gt-marker',
      html: '<div class="marker-num">' + n + '</div>',
      iconSize: [27, 27],
      iconAnchor: [13, 13],
      popupAnchor: [0, -14]
    });
  }

  function setDistance(km, mapped) {
    if (mapped < 2) {
      distanceEl.textContent = 'Not available (fewer than two mapped locations)';
      return;
    }
    distanceEl.textContent = km.toLocaleString('en') + ' km (straight lines between locations)';
  }

  function buildMap(L, coords) {
    var map = L.map(mapEl);
    addTiles(L, map);

    var points = coords.map(function (c) { return [c.lat, c.lng]; });
    coords.forEach(function (c, i) {
      var marker = L.marker(points[i], { icon: numberIcon(L, i + 1), title: pretty(c.location), keyboard: true });
      var box = document.createElement('div');
      var title = document.createElement('strong');
      title.textContent = (i + 1) + '. ' + pretty(c.location);
      box.appendChild(title);
      marker.bindPopup(box).addTo(map);
    });

    if (points.length > 1) {
      L.polyline(points, { color: '#0b5cad', weight: 3, opacity: 0.85 }).addTo(map);
      map.fitBounds(points, { padding: [30, 30] });
    } else {
      map.setView(points[0], 6);
    }
    return { map: map, points: points };
  }

  // ---- the "Play tour" animation ----

  function setupAnimation(L, built, coords) {
    var points = built.points;
    if (points.length < 2) return;

    var legs = points.length - 1;
    var legMs = reduceMotion ? 900 : Math.min(1500, 40000 / legs);
    var walker = L.marker(points[0], {
      icon: L.divIcon({ className: 'gt-marker', html: '<div class="marker-walker"></div>', iconSize: [20, 20], iconAnchor: [10, 10] }),
      interactive: false,
      keyboard: false,
      zIndexOffset: 1000
    });

    var state = 'idle'; // idle | playing | paused
    var leg = 0;
    var t = 0;
    var last = null;
    var raf = null;

    function ease(x) { return x < 0.5 ? 2 * x * x : 1 - Math.pow(-2 * x + 2, 2) / 2; }

    function announce() {
      playInfo.textContent = 'Leg ' + (leg + 1) + ' of ' + legs + ': ' +
        pretty(coords[leg].location) + ' → ' + pretty(coords[leg + 1].location);
    }

    function setButton() {
      playBtn.textContent = state === 'playing' ? '⏸ Pause' : state === 'paused' ? '▶ Resume' : '▶ Play tour';
    }

    function finish() {
      state = 'idle';
      leg = 0;
      t = 0;
      last = null;
      walker.setLatLng(points[points.length - 1]);
      playInfo.textContent = 'Tour finished: ' + coords.length + ' locations visited.';
      playBtn.textContent = '↺ Replay tour';
    }

    function frame(now) {
      if (state !== 'playing') return;
      if (last === null) last = now;
      t += (now - last) / legMs;
      last = now;

      while (t >= 1 && leg < legs - 1) {
        t -= 1;
        leg++;
        announce();
      }
      if (t >= 1) {
        finish();
        return;
      }
      var p = reduceMotion ? 0 : ease(t); // reduced motion: hop from place to place
      var a = points[leg];
      var b = points[leg + 1];
      walker.setLatLng([a[0] + (b[0] - a[0]) * p, a[1] + (b[1] - a[1]) * p]);
      raf = requestAnimationFrame(frame);
    }

    playBtn.disabled = false;
    playBtn.addEventListener('click', function () {
      if (state === 'playing') {
        state = 'paused';
        cancelAnimationFrame(raf);
        last = null;
        setButton();
        return;
      }
      if (state === 'idle') {
        leg = 0;
        t = 0;
        walker.setLatLng(points[0]);
        walker.addTo(built.map);
        announce();
      }
      state = 'playing';
      last = null;
      setButton();
      raf = requestAnimationFrame(frame);
    });
  }

  // ---- fetching ----

  function showMessage(text) { statusEl.textContent = text; }

  function start(polls) {
    fetch('/api/artist/' + encodeURIComponent(artistId) + '/geo')
      .then(function (res) {
        if (!res.ok) throw new Error('HTTP ' + res.status);
        return res.json();
      })
      .then(function (data) {
        if (!data.ready) {
          showMessage('Looking up concert locations… this can take a few minutes after the server starts.');
          distanceEl.textContent = 'Calculating…';
          if (polls < MAX_POLLS) setTimeout(function () { start(polls + 1); }, POLL_MS);
          else showMessage('Locations are still not available. Please reload the page later.');
          return;
        }
        if (data.coordinates.length === 0) {
          showMessage('No map locations are available for this artist.');
          mapEl.hidden = true;
          setDistance(0, 0);
          return;
        }
        showMessage('Showing ' + data.coordinates.length + ' of ' + listedLocations + ' concert locations.' +
          (data.coordinates.length < listedLocations ? ' The others could not be found on the map.' : ''));
        setDistance(data.distance_km, data.coordinates.length);
        return loadLeaflet().then(function (L) {
          var built = buildMap(L, data.coordinates);
          setupAnimation(L, built, data.coordinates);
        });
      })
      .catch(function () {
        showMessage('The map could not be loaded. The concert list below still shows every location.');
        distanceEl.textContent = 'Not available';
      });
  }

  start(0);
})();