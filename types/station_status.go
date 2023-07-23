package types

type StationStatus struct {
	Data struct {
		Stations []Station `json:"stations"`
	} `json:"data"`
	LastUpdated int `json:"last_updated"`
	TTL         int `json:"ttl"`
}

type Station struct {
	LegacyID               string `json:"legacy_id"`
	NumScootersUnavailable int    `json:"num_scooters_unavailable,omitempty"`
	LastReported           int    `json:"last_reported"`
	StationStatus          string `json:"station_status"`
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
}
