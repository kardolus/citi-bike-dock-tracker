#!/usr/bin/env python3
"""Build configs/paris/neighborhoods.json for Vélib' Métropole (Paris region).

Vélib' spans Paris + the petite couronne, so two-source first-match precedence:
  1. the 20 Paris arrondissements  → area = paris, neighborhood = "Paris Ne".
  2. communes of Hauts-de-Seine (92) / Seine-Saint-Denis (93) / Val-de-Marne (94)
     → one neighborhood per commune, area = the department.
Stations outside every polygon snap to the nearest centroid (a handful of outer-ring ones).

Same ray-cast as the Go ingester; rings simplified to fit the 1 MB ConfigMap limit. Reads
cached inputs in /tmp (Paris GBFS station_information, arrondissements GeoJSON, the three
commune GeoJSONs from gregoiredavid/france-geojson). Emits neighborhoods.json + meta.json +
a validation report.   Run:  python3 scripts/build_paris.py
"""
import json
import math
import os
import re
from collections import defaultdict

HERE = os.path.dirname(__file__)
OUTDIR = os.path.join(HERE, "..", "configs", "paris")
ARR = os.environ.get("PARIS_ARR", "/tmp/paris_arr.geojson")
STATIONS = os.environ.get("PARIS_STATIONS", "/tmp/paris_si.json")
DEPTS = [("92-hauts-de-seine", "hauts-de-seine"),
         ("93-seine-saint-denis", "seine-saint-denis"),
         ("94-val-de-marne", "val-de-marne")]
_MIN_STEP = 0.0002  # ~20 m decimation


def slugify(s):
    return re.sub(r"[^a-z0-9]+", "-", s.lower()).strip("-")


def ray_inside(lat, lon, ring):
    inside, n, j = False, len(ring), len(ring) - 1
    for i in range(n):
        yi, xi = ring[i]
        yj, xj = ring[j]
        if ((yi > lat) != (yj > lat)) and (lon < (xj - xi) * (lat - yi) / (yj - yi) + xi):
            inside = not inside
        j = i
    return inside


def _simplify(ring):
    out = []
    for lat, lon in ring:
        lat, lon = round(lat, 5), round(lon, 5)
        if not out or abs(lat - out[-1][0]) + abs(lon - out[-1][1]) >= _MIN_STEP:
            out.append([lat, lon])
    if len(out) >= 3 and out[0] != out[-1]:
        out.append(out[0])
    return out if len(out) >= 4 else [[round(la, 5), round(lo, 5)] for la, lo in ring]


def grings(g):
    polys = g["coordinates"] if g["type"] == "MultiPolygon" else [g["coordinates"]]
    return [_simplify([[lat, lon] for lon, lat in poly[0]]) for poly in polys]


def load_sources():
    out, seen = [], set()

    def add(slug, display, area, rings):
        base, i = slug, 2
        while slug in seen:
            slug = f"{base}-{i}"; i += 1
        seen.add(slug); out.append((slug, display, area, rings))

    for f in json.load(open(ARR))["features"]:
        n = int(f["properties"]["c_ar"])
        disp = f"Paris {n}{'er' if n == 1 else 'e'}"
        add(f"paris-{n}", disp, "paris", grings(f["geometry"]))
    for fname, area in DEPTS:
        for f in json.load(open(f"/tmp/communes-{fname}.geojson"))["features"]:
            nm = f["properties"]["nom"]
            add(slugify(nm), nm, area, grings(f["geometry"]))
    return out


def main():
    sources = load_sources()
    n_arr = sum(1 for s in sources if s[2] == "paris")
    print(f"loaded {len(sources)} candidates ({n_arr} arrondissements, {len(sources)-n_arr} communes)")
    stations = [s for s in json.load(open(STATIONS))["data"]["stations"]
                if s.get("lat") is not None and s.get("lon") is not None]
    members, meta, leftover = defaultdict(list), {}, []
    for st in stations:
        lat, lon = st["lat"], st["lon"]
        hit = next((s for s in sources
                    if any(ray_inside(lat, lon, r) for r in s[3])), None)
        if hit is None:
            leftover.append(st); continue
        members[hit[0]].append((lat, lon)); meta[hit[0]] = hit[1:]
    cent = {s: (sum(p[0] for p in m) / len(m), sum(p[1] for p in m) / len(m)) for s, m in members.items()}
    src = {s[0]: s for s in sources}

    def nearest(lat, lon):
        best, bd = None, 1e18
        for s, (cl, co) in cent.items():
            d = (lat - cl) ** 2 + ((lon - co) * math.cos(math.radians(lat))) ** 2
            if d < bd:
                bd, best = d, s
        return best
    snapped = 0
    for st in leftover:
        s = nearest(st["lat"], st["lon"])
        if s:
            members[s].append((st["lat"], st["lon"])); meta[s] = src[s][1:]; snapped += 1
    order = {s[0]: i for i, s in enumerate(sources)}
    out = []
    for slug in sorted(members, key=lambda s: order[s]):
        disp, area, rings = meta[slug]
        pts = members[slug]
        out.append({"slug": slug, "display": disp, "area": area,
                    "centroid": [round(sum(p[0] for p in pts) / len(pts), 5),
                                 round(sum(p[1] for p in pts) / len(pts), 5)],
                    "count": len(pts), "rings": rings})
    os.makedirs(OUTDIR, exist_ok=True)
    json.dump(out, open(os.path.join(OUTDIR, "neighborhoods.json"), "w"))
    json.dump([{k: n[k] for k in ("slug", "display", "area", "centroid", "count")} for n in out],
              open(os.path.join(OUTDIR, "neighborhoods.meta.json"), "w"), indent=0)
    by_area = defaultdict(lambda: [0, 0])
    for n in out:
        by_area[n["area"]][0] += 1; by_area[n["area"]][1] += n["count"]
    print(f"\nwrote {len(out)} neighborhoods covering {sum(n['count'] for n in out)}/{len(stations)} "
          f"stations ({snapped} snapped):")
    for a in sorted(by_area):
        nh, ns = by_area[a]
        print(f"  {a:18s} {nh:3d} neighborhoods  {ns:4d} stations")


if __name__ == "__main__":
    main()
