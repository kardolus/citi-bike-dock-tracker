#!/usr/bin/env python3
"""Build configs/dc/neighborhoods.json for Capital Bikeshare (Washington, DC region).

Capital Bikeshare's ~849 stations span 8 jurisdictions, and DC's "Neighborhood
Clusters" only tile residential DC (downtown / the Mall / waterfront sit in gaps).
So the geography is two-source, first-match precedence:
  1. DC Neighborhood Clusters  → area = washington-dc, fine neighborhoods.
  2. Suburban county polygons (Arlington, Alexandria, Fairfax {+ city}, Falls Church,
     Montgomery, Prince George's) → each one neighborhood, area = the jurisdiction.
Stations outside every polygon (downtown-DC cluster gaps) snap to the nearest centroid.

Same ray-cast point-in-polygon as the Go ingester, so the offline assignment matches
runtime. Emits neighborhoods.json (rings) + neighborhoods.meta.json (no rings) + a
validation report. Inputs are cached in /tmp by the fetch steps in the runbook; this
reads them (DC clusters, filtered counties, DC GBFS station_information).

Run:  python3 scripts/build_dc.py
"""
import json
import math
import os
import re
from collections import defaultdict

HERE = os.path.dirname(__file__)
REPO = os.path.join(HERE, "..")
OUTDIR = os.path.join(REPO, "configs", "dc")
CLUSTERS = os.environ.get("DC_CLUSTERS", "/tmp/dc_clusters.geojson")
COUNTIES = os.environ.get("DC_COUNTIES", "/tmp/dc_counties.json")
STATIONS = os.environ.get("DC_STATIONS", "/tmp/dc_si.json")


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


def contains(rings, lat, lon):
    return any(ray_inside(lat, lon, r) for r in rings)


# Boundary precision we keep: ~5 decimals (≈1 m) rounding + drop points within ~20 m of
# the previous kept point. Point-in-polygon at city scale doesn't need more, and it keeps
# neighborhoods.json under the 1 MB k8s ConfigMap limit (county rings are otherwise huge).
_MIN_STEP = 0.0002  # degrees ≈ 20 m


def _simplify(ring):
    out = []
    for lat, lon in ring:
        lat, lon = round(lat, 5), round(lon, 5)
        if not out or abs(lat - out[-1][0]) + abs(lon - out[-1][1]) >= _MIN_STEP:
            out.append([lat, lon])
    if len(out) >= 3 and out[0] != out[-1]:
        out.append(out[0])  # keep the ring closed
    return out if len(out) >= 4 else [[round(la, 5), round(lo, 5)] for la, lo in ring]


def geom_rings(g):
    """GeoJSON Polygon/MultiPolygon → list of simplified outer rings as [lat, lon]."""
    polys = g["coordinates"] if g["type"] == "MultiPolygon" else [g["coordinates"]]
    return [_simplify([[lat, lon] for lon, lat in poly[0]]) for poly in polys]


def load_sources():
    out = []  # (slug, display, area, rings) in precedence order
    seen = set()

    def add(slug, display, area, rings):
        base, i = slug, 2
        while slug in seen:
            slug = f"{base}-{i}"; i += 1
        seen.add(slug)
        out.append((slug, display, area, rings))

    # 1. DC Neighborhood Clusters → fine DC neighborhoods.
    for f in json.load(open(CLUSTERS))["features"]:
        p = f["properties"]
        names = (p.get("NBH_NAMES") or p.get("NAME") or "").split(",")
        display = names[0].strip() or p.get("NAME", "DC")
        add(slugify(display), display, "washington-dc", geom_rings(f["geometry"]))

    # 2. Suburban counties → one neighborhood per jurisdiction.
    cj = json.load(open(COUNTIES))
    names = cj["names"]
    for f in cj["features"]:
        nm = names[f["id"]]
        add(slugify(nm), nm, slugify(nm), geom_rings(f["geometry"]))
    return out


def main():
    sources = load_sources()
    n_cluster = sum(1 for s in sources if s[2] == "washington-dc")
    print(f"loaded {len(sources)} candidates ({n_cluster} DC clusters, {len(sources)-n_cluster} suburban counties)")

    stations = [s for s in json.load(open(STATIONS))["data"]["stations"]
                if s.get("lat") is not None and s.get("lon") is not None]
    members = defaultdict(list)
    meta = {}
    leftover = []

    for st in stations:
        lat, lon = st["lat"], st["lon"]
        hit = next((s for s in sources if contains(s[3], lat, lon)), None)
        if hit is None:
            leftover.append(st); continue
        slug, disp, area, rings = hit
        members[slug].append((lat, lon)); meta[slug] = (disp, area, rings)

    cent = {s: (sum(p[0] for p in m) / len(m), sum(p[1] for p in m) / len(m))
            for s, m in members.items()}
    src_by_slug = {s[0]: s for s in sources}

    def nearest(lat, lon):
        best, bd = None, 1e18
        for s, (clat, clon) in cent.items():
            d = (lat - clat) ** 2 + ((lon - clon) * math.cos(math.radians(lat))) ** 2
            if d < bd:
                bd, best = d, s
        return best, math.sqrt(bd) * 111_000  # rough metres

    snapped, far = 0, []
    for st in leftover:
        slug, dist = nearest(st["lat"], st["lon"])
        if slug is None:
            continue
        _, disp, area, rings = src_by_slug[slug]
        members[slug].append((st["lat"], st["lon"])); meta[slug] = (disp, area, rings)
        snapped += 1
        if dist > 1500:
            far.append((st["name"].strip(), round(dist)))
    unmapped = len(leftover) - snapped

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
        by_area[n["area"]][0] += 1
        by_area[n["area"]][1] += n["count"]
    print(f"\nwrote {len(out)} neighborhoods covering {sum(n['count'] for n in out)}/{len(stations)} "
          f"stations ({snapped} snapped to nearest, {unmapped} unmapped):")
    for area in sorted(by_area):
        nh, ns = by_area[area]
        print(f"  {area:16s} {nh:3d} neighborhoods  {ns:4d} stations")
    if far:
        print(f"\n⚠ {len(far)} stations snapped >1.5km (review polygon coverage):")
        for nm, d in sorted(far, key=lambda x: -x[1])[:12]:
            print(f"    {d:5d}m  {nm[:42]}")


if __name__ == "__main__":
    main()
