package client

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

//go:embed neighborhoods.json
var neighborhoodsJSON []byte

// Neighborhood is a named area defined by one or more polygon rings of [lat, lon]
// points, grouped under an `area` (borough / Jersey City / Hoboken). The set is
// built by scripts/build_neighborhoods.py from the 29 curated Brooklyn polygons
// (which take precedence), hand-drawn JC/Hoboken polygons, and the NYC 2020 NTAs
// for the other boroughs. `centroid` (mean of member stations) backs the web's
// map bubbles and the nearest-neighbour fallback for points outside every polygon.
type Neighborhood struct {
	Slug     string         `json:"slug"`
	Display  string         `json:"display"`
	Area     string         `json:"area"`
	Centroid [2]float64     `json:"centroid"` // [lat, lon]
	Count    int            `json:"count"`
	Rings    [][][2]float64 `json:"rings"`   // outer rings of [lat, lon] points
	Polygon  [][2]float64   `json:"polygon"` // legacy single ring (back-compat)
}

// LoadNeighborhoods returns the embedded neighborhood set (the NYC default; kept
// as a fallback for the original Citi Bike deployment and the test suite).
func LoadNeighborhoods() ([]Neighborhood, error) {
	return parseNeighborhoods(neighborhoodsJSON)
}

// LoadNeighborhoodsFromFile reads the neighborhood set from a file on disk — the
// per-city path (a mounted ConfigMap) that makes one image serve every city. It is
// the runtime source when NEIGHBORHOODS_PATH is set; the embed is only the fallback.
func LoadNeighborhoodsFromFile(path string) ([]Neighborhood, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read neighborhoods %q: %w", path, err)
	}
	return parseNeighborhoods(data)
}

func parseNeighborhoods(data []byte) ([]Neighborhood, error) {
	var ns []Neighborhood
	if err := json.Unmarshal(data, &ns); err != nil {
		return nil, fmt.Errorf("parse neighborhoods.json: %w", err)
	}
	if len(ns) == 0 {
		return nil, fmt.Errorf("neighborhoods.json is empty")
	}
	return ns, nil
}

// rings returns the neighborhood's polygon rings (supporting the legacy single
// `polygon` form).
func (n Neighborhood) rings() [][][2]float64 {
	if len(n.Rings) > 0 {
		return n.Rings
	}
	if len(n.Polygon) > 0 {
		return [][][2]float64{n.Polygon}
	}
	return nil
}

// contains reports whether (lat, lon) lies inside any of the neighborhood's rings
// (ray casting; holes are not modelled).
func (n Neighborhood) contains(lat, lon float64) bool {
	for _, ring := range n.rings() {
		if ringContains(ring, lat, lon) {
			return true
		}
	}
	return false
}

func ringContains(p [][2]float64, lat, lon float64) bool {
	inside := false
	j := len(p) - 1
	for i := 0; i < len(p); i++ {
		yi, xi := p[i][0], p[i][1]
		yj, xj := p[j][0], p[j][1]
		if ((yi > lat) != (yj > lat)) && (lon < (xj-xi)*(lat-yi)/(yj-yi)+xi) {
			inside = !inside
		}
		j = i
	}
	return inside
}

// assignNeighborhood returns the slug of the first neighborhood containing the
// point (first-match/file order resolves overlaps); for points outside every
// polygon it snaps to the nearest neighborhood by centroid (matching the build
// script), so every station in the service area gets a neighborhood. Returns ""
// only if the set has no usable centroids.
func assignNeighborhood(ns []Neighborhood, lat, lon float64) string {
	for _, n := range ns {
		if n.contains(lat, lon) {
			return n.Slug
		}
	}
	best, bestD := "", math.Inf(1)
	for _, n := range ns {
		if n.Centroid[0] == 0 && n.Centroid[1] == 0 {
			continue
		}
		dLat := lat - n.Centroid[0]
		dLon := (lon - n.Centroid[1]) * math.Cos(lat*math.Pi/180)
		if d := dLat*dLat + dLon*dLon; d < bestD {
			bestD, best = d, n.Slug
		}
	}
	return best
}
