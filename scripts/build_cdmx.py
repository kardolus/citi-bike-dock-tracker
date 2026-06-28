#!/usr/bin/env python3
"""Build configs/cdmx/neighborhoods.json for Ecobici (Mexico City).

Ecobici is a compact ~681-station central system. We assign each station to its colonia
using the curated Ecobici colonia borders (52 colonias: Roma, Condesa, Centro, Polanco,
Del Valle, Nápoles, …) from jjsantos01/mi-movilidad. Those border features are closed
LineStrings, used here directly as polygon rings. Area is a single "Ciudad de México"
(boroughs don't fit a compact central system); the web defaults its landing to the colonia
view. Alcaldía grouping is a possible future refinement.

Same ray-cast as the Go ingester; rings simplified for the ConfigMap limit. Input: the
colonias_ecobici_bordes GeoJSON + the Ecobici GBFS station_information. Emits
neighborhoods.json + meta.json + a report.   Run:  python3 scripts/build_cdmx.py
"""
import json
import math
import os
import re
from collections import defaultdict

HERE = os.path.dirname(__file__)
OUTDIR = os.path.join(HERE, "..", "configs", "cdmx")
COLONIAS = os.environ.get("CDMX_COLONIAS", "/tmp/cdmx_colonias.geojson")
STATIONS = os.environ.get("CDMX_STATIONS", "/tmp/cdmx_si.json")
AREA = "ciudad-de-mexico"
_MIN_STEP = 0.0002

_ACCENTS = {"á": "a", "é": "e", "í": "i", "ó": "o", "ú": "u", "ñ": "n"}


def slugify(s):
    s = "".join(_ACCENTS.get(c, c) for c in s.lower())
    return re.sub(r"[^a-z0-9]+", "-", s).strip("-")


def titlecase(s):
    return " ".join(w.capitalize() for w in s.split())


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


def ring_of(feat):
    g = feat["geometry"]
    if g["type"] == "LineString":
        coords = g["coordinates"]
    elif g["type"] == "MultiLineString":
        coords = g["coordinates"][0]
    else:
        coords = g["coordinates"][0]
    return _simplify([[lat, lon] for lon, lat in coords])


def load_sources():
    out, seen = [], set()
    for f in json.load(open(COLONIAS))["features"]:
        nm = f["properties"]["nombre"]
        slug = slugify(nm)
        base, i = slug, 2
        while slug in seen:
            slug = f"{base}-{i}"; i += 1
        seen.add(slug)
        out.append((slug, titlecase(nm), AREA, [ring_of(f)]))
    return out


def main():
    sources = load_sources()
    print(f"loaded {len(sources)} Ecobici colonias")
    stations = [s for s in json.load(open(STATIONS))["data"]["stations"]
                if s.get("lat") is not None and s.get("lon") is not None]
    members, meta, leftover = defaultdict(list), {}, []
    for st in stations:
        lat, lon = st["lat"], st["lon"]
        hit = next((s for s in sources if any(ray_inside(lat, lon, r) for r in s[3])), None)
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
    snapped, far = 0, 0
    for st in leftover:
        s = nearest(st["lat"], st["lon"])
        if s:
            cl, co = cent[s]
            if math.hypot(st["lat"] - cl, st["lon"] - co) * 111_000 > 2000:
                far += 1
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
    print(f"\nwrote {len(out)} colonias covering {sum(n['count'] for n in out)}/{len(stations)} "
          f"stations ({snapped} snapped, {far} >2km):")
    for n in sorted(out, key=lambda n: -n["count"])[:12]:
        print(f"  {n['display']:26s} {n['count']:4d} stations")


if __name__ == "__main__":
    main()
