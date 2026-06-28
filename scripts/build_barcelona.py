#!/usr/bin/env python3
"""Build configs/barcelona/neighborhoods.json for Bicing (Barcelona).

Bicing's ~542 docked stations sit inside Barcelona's official administrative geography:
  - neighborhood = barri (73 of them, e.g. el Raval, la Barceloneta, Sant Antoni),
  - area = districte (the 10 city districts: Ciutat Vella, Eixample, Gràcia, …) so the
    Everywhere view rolls up to districts instead of one blob.
Boundaries are the City of Barcelona open-data polygons (WGS84) via the martgnz/bcn-geodata
mirror; each barri carries a DISTRICTE code that maps to the district name. District slugs are
accent-stripped, so the web needs AREA_OVERRIDES to restore the accented display (printed below).

Same ray-cast as the Go ingester; rings simplified for the ConfigMap limit. Stations outside
every barri snap to the nearest centroid (Bicing barely extends past the city line).
Run:  python3 scripts/build_barcelona.py
"""
import json
import math
import os
import re
import unicodedata
import urllib.request
from collections import defaultdict

HERE = os.path.dirname(__file__)
OUTDIR = os.path.join(HERE, "..", "configs", "barcelona")
BARRIS = os.environ.get("BCN_BARRIS", "/tmp/barris.json")
DISTRICTES = os.environ.get("BCN_DISTRICTES", "/tmp/districtes.json")
GBFS = "https://barcelona.publicbikesystem.net/customer/gbfs/v2/en/station_information"
_MIN_STEP = 0.0002


def slugify(s):
    s = "".join(c for c in unicodedata.normalize("NFKD", s) if not unicodedata.combining(c))
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
    # district code -> display name (Ciutat Vella, Eixample, …)
    dist = {}
    for f in json.load(open(DISTRICTES))["features"]:
        p = f["properties"]
        dist[p["DISTRICTE"]] = p["NOM"]
    out = []
    for f in json.load(open(BARRIS))["features"]:
        p = f["properties"]
        area_name = dist.get(p["DISTRICTE"], "Barcelona")
        out.append((slugify(p["NOM"]), p["NOM"], slugify(area_name), area_name, grings(f["geometry"])))
    return out, dist


def fetch_stations():
    req = urllib.request.Request(GBFS, headers={"User-Agent": "dockscan/1.0 (+https://kardol.us)"})
    d = json.load(urllib.request.urlopen(req, timeout=20))["data"]["stations"]
    return [s for s in d if s.get("lat") is not None and s.get("lon") is not None]


def main():
    sources, dist = load_sources()
    print(f"loaded {len(sources)} barris across {len(dist)} districts")
    stations = fetch_stations()
    # source tuple: (slug, display, area_slug, area_name, rings)
    members, meta, leftover = defaultdict(list), {}, []
    for st in stations:
        lat, lon = st["lat"], st["lon"]
        hit = next((s for s in sources if any(ray_inside(lat, lon, r) for r in s[4])), None)
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
    out, area_disp = [], {}
    for slug in sorted(members, key=lambda s: order[s]):
        disp, area_slug, area_name, rings = meta[slug]
        area_disp[area_slug] = area_name
        pts = members[slug]
        out.append({"slug": slug, "display": disp, "area": area_slug,
                    "centroid": [round(sum(p[0] for p in pts) / len(pts), 5),
                                 round(sum(p[1] for p in pts) / len(pts), 5)],
                    "count": len(pts), "rings": rings})
    os.makedirs(OUTDIR, exist_ok=True)
    json.dump(out, open(os.path.join(OUTDIR, "neighborhoods.json"), "w"))
    json.dump([{k: n[k] for k in ("slug", "display", "area", "centroid", "count")} for n in out],
              open(os.path.join(OUTDIR, "neighborhoods.meta.json"), "w"), indent=0)
    by_area = defaultdict(lambda: [0, 0])
    for n in out:
        by_area[n["area"]][0] += 1
        by_area[n["area"]][1] += n["count"]
    sz = os.path.getsize(os.path.join(OUTDIR, "neighborhoods.json"))
    print(f"\nwrote {len(out)} barris covering {sum(n['count'] for n in out)}/{len(stations)} "
          f"stations ({snapped} snapped), {sz/1024:.0f} KB:")
    for a in sorted(by_area, key=lambda a: -by_area[a][1]):
        nh, ns = by_area[a]
        print(f"  {area_disp[a]:22s} {nh:3d} barris  {ns:4d} stations")
    # AREA_OVERRIDES for the web (accent-stripped slug -> accented district name)
    print("\nAREA_OVERRIDES (paste into bicing.yaml areaOverrides):")
    print(" ", json.dumps(area_disp, ensure_ascii=False, sort_keys=True))


if __name__ == "__main__":
    main()
