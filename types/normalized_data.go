package types

import "time"

type NormalizedStationData struct {
	Stations  []NormalizedStation `json:"stations"`
	TimeStamp time.Time           `json:"timeStamp"`
}

type NormalizedStationDataTS struct {
	Station   NormalizedStation `json:"station"`
	TimeStamp time.Time         `json:"timestamp"`
}

type NormalizedStation struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	Longitude           float64 `json:"longitude"`
	Latitude            float64 `json:"latitude"`
	Location            string  `json:"location"`
	Status              string  `json:"status"`
	BikesAvailable      int     `json:"bikesAvailable"`
	EBikesAvailable     int     `json:"eBikesAvailable"`
	BikesDisabled       int     `json:"bikesDisabled"`
	DocksAvailable      int     `json:"docksAvailable"`
	DocksDisabled       int     `json:"docksDisabled"`
	ScootersAvailable   int     `json:"scootersAvailable,omitempty"`
	ScootersUnavailable int     `json:"scootersUnavailable,omitempty"`
	IsReturning         bool    `json:"isReturning"`
	IsRenting           bool    `json:"isRenting"`
	IsInstalled         bool    `json:"isInstalled"`
}
