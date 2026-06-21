package client

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed neighborhoods.json
var neighborhoodsJSON []byte

// Neighborhood is a named area defined by a polygon ring of [lat, lon] points.
// The curated set (Red Hook + Brooklyn neighbors) lives in neighborhoods.json;
// boundaries were drawn from street edges and validated against live GBFS
// station coordinates (the official NYC NTAs lump Red Hook / Carroll Gardens /
// Cobble Hill / Gowanus into one area, so they can't be used directly).
type Neighborhood struct {
	Slug    string       `json:"slug"`
	Display string       `json:"display"`
	Polygon [][2]float64 `json:"polygon"` // ring of [lat, lon] points
}

// LoadNeighborhoods returns the embedded curated neighborhood set.
func LoadNeighborhoods() ([]Neighborhood, error) {
	var ns []Neighborhood
	if err := json.Unmarshal(neighborhoodsJSON, &ns); err != nil {
		return nil, fmt.Errorf("parse neighborhoods.json: %w", err)
	}
	if len(ns) == 0 {
		return nil, fmt.Errorf("neighborhoods.json is empty")
	}
	return ns, nil
}

// contains reports whether (lat, lon) lies inside the polygon (ray casting).
func (n Neighborhood) contains(lat, lon float64) bool {
	p := n.Polygon
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
// point, or "" if none. First-match order (file order) resolves any overlaps.
func assignNeighborhood(ns []Neighborhood, lat, lon float64) string {
	for _, n := range ns {
		if n.contains(lat, lon) {
			return n.Slug
		}
	}
	return ""
}
