package types

type StationStatus struct {
	Data struct {
		Stations []Station `json:"stations"`
	} `json:"data"`
	LastUpdated int `json:"last_updated"`
	TTL         int `json:"ttl"`
}

type Station struct {
	NumScootersUnavailable int    `json:"num_scooters_unavailable,omitempty"`
	LastReported           int    `json:"last_reported"`
	EightdHasAvailableKeys bool   `json:"eightd_has_available_keys"`
	IsReturning            int    `json:"is_returning"`
	StationID              string `json:"station_id"`
	NumEbikesAvailable     int    `json:"num_ebikes_available"`
	NumScootersAvailable   int    `json:"num_scooters_available,omitempty"`
	IsRenting              int    `json:"is_renting"`
	NumBikesDisabled       int    `json:"num_bikes_disabled"`
	IsInstalled            int    `json:"is_installed"`
	NumDocksDisabled       int    `json:"num_docks_disabled"`
	NumBikesAvailable      int    `json:"num_bikes_available"`
	NumDocksAvailable      int    `json:"num_docks_available"`
	// NumBikesAvailableTypes is the Smovengo/Vélib' way of reporting the
	// mechanical/e-bike split (e.g. [{"mechanical":1},{"ebike":8}]); Lyft feeds
	// (Citi Bike, Capital Bikeshare, Ecobici) use num_ebikes_available instead.
	NumBikesAvailableTypes []map[string]int `json:"num_bikes_available_types,omitempty"`
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
	sum := 0
	for _, m := range s.NumBikesAvailableTypes {
		sum += m["ebike"]
	}
	return sum
}
