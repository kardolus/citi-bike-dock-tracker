package types

import "strings"

// VehicleTypes models a GBFS vehicle_types.json feed (PBSC/Bicing). It exists so
// the ingester can learn which vehicle_type_ids are e-bikes — the station_status
// feed only carries opaque ids + counts in vehicle_types_available.
type VehicleTypes struct {
	Data struct {
		VehicleTypes []struct {
			VehicleTypeID  string `json:"vehicle_type_id"`
			FormFactor     string `json:"form_factor"`
			PropulsionType string `json:"propulsion_type"`
		} `json:"vehicle_types"`
	} `json:"data"`
}

// ElectricBicycleTypes returns the set of vehicle_type_ids that count as e-bikes:
// form_factor "bicycle" with an electric propulsion (electric / electric_assist).
// Scooters (e.g. Bicing's CHLOE) and mechanical bikes are excluded.
func (v VehicleTypes) ElectricBicycleTypes() map[string]bool {
	out := make(map[string]bool)
	for _, t := range v.Data.VehicleTypes {
		if t.FormFactor == "bicycle" && strings.HasPrefix(t.PropulsionType, "electric") {
			out[t.VehicleTypeID] = true
		}
	}
	return out
}
