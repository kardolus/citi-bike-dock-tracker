#!/usr/bin/env python3
"""Build configs/london/neighborhoods.json for Santander Cycles (London).

Santander Cycles' ~799 docking stations sit in central/inner London. Geography:
  - neighborhood = London borough (e.g. Westminster, Camden, Tower Hamlets),
  - area = the GLA London-Plan sub-region (Central / North / East / South / West) so the
    Everywhere view rolls up sensibly. The borough→sub-region map below is the 2011 London
    Plan grouping (sourced); area slugs carry "-london" so titlecasing yields "Central London"
    etc. with no AREA_OVERRIDES (the Chicago "-side" trick).

Stations come from TfL's BikePoint API (lat/lon). Boroughs from the London Datastore borough
GeoJSON (WGS84). Same ray-cast + ring-simplify as the other builders; stations outside every
borough snap to the nearest centroid. Run:  python3 scripts/build_london.py
"""
import json
import math
import os
import re
import urllib.request
from collections import defaultdict

HERE = os.path.dirname(__file__)
OUTDIR = os.path.join(HERE, "..", "configs", "london")
BOROUGHS = os.environ.get("LON_BOROUGHS", "/tmp/lon_boroughs.geojson")
BIKEPOINT = "https://api.tfl.gov.uk/bikepoint"
_MIN_STEP = 0.0002

# Borough -> GLA sub-region (2011 London Plan). Area slug = "<sub-region>-london".
_SUBREGION = {
    "City of London": "Central", "Kensington and Chelsea": "Central", "Camden": "Central",
    "Islington": "Central", "Lambeth": "Central", "Southwark": "Central", "Westminster": "Central",
    "Barking and Dagenham": "East", "Bexley": "East", "Greenwich": "East", "Hackney": "East",
    "Havering": "East", "Lewisham": "East", "Newham": "East", "Redbridge": "East",
    "Tower Hamlets": "East", "Waltham Forest": "East",
    "Barnet": "North", "Enfield": "North", "Haringey": "North",
    "Bromley": "South", "Croydon": "South", "Kingston upon Thames": "South", "Merton": "South",
    "Richmond upon Thames": "South", "Sutton": "South", "Wandsworth": "South",
    "Brent": "West", "Ealing": "West", "Hammersmith and Fulham": "West", "Harrow": "West",
    "Hillingdon": "West", "Hounslow": "West",
}


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
    out = []
    for f in json.load(open(BOROUGHS))["features"]:
        name = f["properties"]["name"]
        sub = _SUBREGION.get(name, "Central")
        out.append((slugify(name), name, f"{sub.lower()}-london", f"{sub} London", grings(f["geometry"])))
    return out


def fetch_stations():
    req = urllib.request.Request(BIKEPOINT, headers={"User-Agent": "dockscan/1.0 (+https://kardol.us)"})
    d = json.load(urllib.request.urlopen(req, timeout=25))
    return [s for s in d if s.get("lat") is not None and s.get("lon") is not None]


def main():
    sources = load_sources()
    print(f"loaded {len(sources)} boroughs across {len({s[2] for s in sources})} sub-regions")
    stations = fetch_stations()
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
    print(f"\nwrote {len(out)} boroughs covering {sum(n['count'] for n in out)}/{len(stations)} "
          f"stations ({snapped} snapped), {sz/1024:.0f} KB:")
    for a in sorted(by_area, key=lambda a: -by_area[a][1]):
        nh, ns = by_area[a]
        print(f"  {area_disp[a]:16s} {nh:2d} boroughs  {ns:3d} stations")


if __name__ == "__main__":
    main()
