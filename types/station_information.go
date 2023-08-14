package types

type StationInformation struct {
	Data struct {
		Stations []StationEntity `json:"stations"`
	} `json:"data"`
	LastUpdated int `json:"last_updated"`
	TTL         int `json:"ttl"`
}

type StationEntity struct {
	ExternalID string  `json:"external_id"`
	Lat        float64 `json:"lat"`
	StationID  string  `json:"station_id"`
	RentalUris struct {
		Ios     string `json:"ios"`
		Android string `json:"android"`
	} `json:"rental_uris"`
	Lon                         float64       `json:"lon"`
	StationType                 string        `json:"station_type"`
	Capacity                    int           `json:"capacity"`
	Name                        string        `json:"name"`
	HasKiosk                    bool          `json:"has_kiosk"`
	EightdHasKeyDispenser       bool          `json:"eightd_has_key_dispenser"`
	ShortName                   string        `json:"short_name"`
	ElectricBikeSurchargeWaiver bool          `json:"electric_bike_surcharge_waiver"`
	EightdStationServices       []interface{} `json:"eightd_station_services"`
	RentalMethods               []string      `json:"rental_methods"`
	RegionID                    string        `json:"region_id,omitempty"`
}
