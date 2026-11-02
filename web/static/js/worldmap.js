// worldmap.js - the "world map" on the home page: every concert location of
// the artists that pass the current filters. The map is created only when the
// user opens it, and Leaflet is downloaded at that moment.
(function () {
  'use strict';

  var toggle = document.getElementById('world-toggle');
  var box = document.getElementById('world');
  var mapEl = document.getElementById('world-map');
  var statusEl = document.getElementById('world-status');
  var grid = document.getElementById('artist-grid');
  if (!toggle || !box || !mapEl || !grid) return;

  var POLL_MS = 3000;
  var MAX_POLLS = 200;
  var pretty = function (raw) { return (window.GT && window.GT.prettyLocation) ? window.GT.prettyLocation(raw) : raw; };

  var names = {}; // artist id -> name, read from the cards
  grid.querySelectorAll('.card').forEach(function (card) { names[card.dataset.id] = card.dataset.name; });

  var L = null;
  var map = null;
  var layer = null;
  var points = null; // answer of /api/geo
  var fitted = false;
  var opened = false;

  function visibleSet() {
    var set = new Set();
    grid.querySelectorAll('.card:not([hidden])').forEach(function (c) { set.add(c.dataset.id); });
    return set;
  }

  function popupFor(point, visibleArtists) {
    var box = document.createElement('div');
    var title = document.createElement('strong');
    title.textContent = pretty(point.location);
    box.appendChild(title);

    var list = document.createElement('ul');
    list.className = 'popup-list';
    visibleArtists.forEach(function (id) {
      var li = document.createElement('li');
      var a = document.createElement('a');
      a.href = '/artist/' + id;
      a.textContent = names[id] || ('Artist ' + id);
      li.appendChild(a);
      list.appendChild(li);
    });
    box.appendChild(list);
    return box;
  }

  // Draws one circle per location that has at least one visible artist.
  function render() {
    if (!map || !points) return;
    layer.clearLayers();
    var visible = visibleSet();
    var latlngs = [];
    var artistsShown = new Set();

    points.forEach(function (point) {
      var ids = point.artist_ids.map(String).filter(function (id) { return visible.has(id); });
      if (ids.length === 0) return;
      ids.forEach(function (id) { artistsShown.add(id); });
      var latlng = [point.lat, point.lng];
      latlngs.push(latlng);
      L.circleMarker(latlng, {
        radius: 5 + Math.min(10, ids.length * 2),
        color: '#ffffff',
        weight: 2,
        fillColor: '#0b5cad',
        fillOpacity: 0.85
      }).bindPopup(function () { return popupFor(point, ids); }).addTo(layer);
    });

    statusEl.textContent = latlngs.length === 0
      ? 'No concert locations for the current filters.'
      : latlngs.length + ' locations · ' + artistsShown.size + ' artists. Click a circle to see who plays there.';

    if (!fitted && latlngs.length > 0) {
      map.fitBounds(latlngs, { padding: [30, 30], maxZoom: 5 });
      fitted = true;
    }
  }

  function fetchPoints(polls) {
    fetch('/api/geo')
      .then(function (res) {
        if (!res.ok) throw new Error('HTTP ' + res.status);
        return res.json();
      })
      .then(function (data) {
        if (!data.ready) {
          statusEl.textContent = 'Looking up concert locations… this can take a few minutes after the server starts.';
          if (polls < MAX_POLLS) setTimeout(function () { fetchPoints(polls + 1); }, POLL_MS);
          return;
        }
        points = data.locations;
        render();
      })
      .catch(function () {
        statusEl.textContent = 'The world map could not be loaded.';
      });
  }

  function open() {
    box.hidden = false;
    toggle.textContent = 'Hide world map';
    toggle.setAttribute('aria-expanded', 'true');
    if (opened) {
      if (map) map.invalidateSize();
      return;
    }
    opened = true;
    statusEl.textContent = 'Loading map…';
    window.GT.loadLeaflet().then(function (leaflet) {
      L = leaflet;
      map = L.map(mapEl, { worldCopyJump: true }).setView([25, 10], 2);
      window.GT.addTiles(L, map);
      layer = L.layerGroup().addTo(map);
      fetchPoints(0);
    }).catch(function () {
      opened = false; // let the user try again
      statusEl.textContent = 'The map library could not be loaded. Check your connection and try again.';
    });
  }

  function close() {
    box.hidden = true;
    toggle.textContent = 'Show world map';
    toggle.setAttribute('aria-expanded', 'false');
  }

  toggle.addEventListener('click', function () { if (box.hidden) open(); else close(); });
  document.addEventListener('filters:change', render);
  document.addEventListener('favorites:change', render);
})();