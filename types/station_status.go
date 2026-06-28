package types

type StationStatus struct {
	Data struct {
		Stations []Station `json:"stations"`
	} `json:"data"`
	LastUpdated any `json:"last_updated"`
	TTL         int `json:"ttl"`
}

type Station struct {
	NumScootersUnavailable int    `json:"num_scooters_unavailable,omitempty"`
	LastReported           any    `json:"last_reported"`
	EightdHasAvailableKeys bool   `json:"eightd_has_available_keys"`
	IsReturning            Flag   `json:"is_returning"`
	StationID              string `json:"station_id"`
	NumEbikesAvailable     int    `json:"num_ebikes_available"`
	NumScootersAvailable   int    `json:"num_scooters_available,omitempty"`
	IsRenting              Flag   `json:"is_renting"`
	NumBikesDisabled       int    `json:"num_bikes_disabled"`
	IsInstalled            Flag   `json:"is_installed"`
	NumDocksDisabled       int    `json:"num_docks_disabled"`
	NumBikesAvailable      int    `json:"num_bikes_available"`
	NumDocksAvailable      int    `json:"num_docks_available"`
	// GBFS v3 (PBSC v3, e.g. BA Ecobici) renames num_bikes_* to num_vehicles_*; read
	// both so Bikes()/Disabled() coalesce across versions. v2 feeds leave these zero.
	NumVehiclesAvailable int `json:"num_vehicles_available"`
	NumVehiclesDisabled  int `json:"num_vehicles_disabled"`
	// NumBikesAvailableTypes is the Smovengo/Vélib' way of reporting the
	// mechanical/e-bike split (e.g. [{"mechanical":1},{"ebike":8}]); Lyft feeds
	// (Citi Bike, Capital Bikeshare, Ecobici) use num_ebikes_available instead.
	NumBikesAvailableTypes []map[string]int `json:"num_bikes_available_types,omitempty"`
	// VehicleTypesAvailable is the PBSC way (Bicing) of reporting the split: a
	// per-vehicle-type count (e.g. [{"vehicle_type_id":"ICONIC","count":14}]).
	// Which type ids are e-bikes comes from vehicle_types.json — see EbikesWith.
	VehicleTypesAvailable []VehicleTypeCount `json:"vehicle_types_available,omitempty"`
}

// VehicleTypeCount is one entry of a PBSC station's vehicle_types_available.
type VehicleTypeCount struct {
	VehicleTypeID string `json:"vehicle_type_id"`
	Count         int    `json:"count"`
}

// Ebikes returns the e-bike count, normalizing across GBFS operator variants:
// it prefers Lyft's num_ebikes_available, falls back to summing the "ebike"
// entries of Smovengo's num_bikes_available_types array, and is 0 when a feed
// exposes neither (e.g. Ecobici — pair with HAS_EBIKES=false to hide the UI).
// For Lyft feeds (no types array) this is exactly num_ebikes_available, so NYC
// stays byte-identical.
func (s Station) Ebikes() int {
	if s.NumEbikesAvailable > 0 {
		return s.NumEbikesAvailable
	}
	return s.ebikeTypeSum()
}

// Bikes is the available-bike count, coalescing GBFS v2 num_bikes_available and v3
// num_vehicles_available (the two are mutually exclusive per feed). For NYC (v2) this
// is exactly num_bikes_available, so it stays byte-identical.
func (s Station) Bikes() int {
	if s.NumBikesAvailable > 0 {
		return s.NumBikesAvailable
	}
	return s.NumVehiclesAvailable
}

// Disabled coalesces v2 num_bikes_disabled and v3 num_vehicles_disabled.
func (s Station) Disabled() int {
	if s.NumBikesDisabled > 0 {
		return s.NumBikesDisabled
	}
	return s.NumVehiclesDisabled
}

func (s Station) ebikeTypeSum() int {
	sum := 0
	for _, m := range s.NumBikesAvailableTypes {
		sum += m["ebike"]
	}
	return sum
}

// EbikesWith is Ebikes plus a third fallback for PBSC feeds (Bicing) that report
// the split only via vehicle_types_available: electric is the set of vehicle_type_ids
// classified as electric bicycles in vehicle_types.json. When electric is nil/empty
// (every other operator) this is identical to Ebikes(), so NYC stays byte-identical.
func (s Station) EbikesWith(electric map[string]bool) int {
	if s.NumEbikesAvailable > 0 {
		return s.NumEbikesAvailable
	}
	if n := s.ebikeTypeSum(); n > 0 {
		return n
	}
	if len(electric) > 0 {
		sum := 0
		for _, v := range s.VehicleTypesAvailable {
			if electric[v.VehicleTypeID] {
				sum += v.Count
			}
		}
		return sum
	}
	return 0
}
