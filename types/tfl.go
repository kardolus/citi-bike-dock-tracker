package types

import (
	"encoding/json"
	"strconv"
)

// TfL's BikePoint API (London Santander Cycles) is NOT GBFS: one endpoint returns an array of
// "Place" objects combining static + live data, with the counts as string key/value pairs in
// additionalProperties. TflToInformation/TflToStatus map it onto the internal GBFS-shaped types
// so everything downstream is unchanged. Gated behind FEED_FORMAT=tfl in the client.

type TflPlace struct {
	ID                   string  `json:"id"`
	CommonName           string  `json:"commonName"`
	Lat                  float64 `json:"lat"`
	Lon                  float64 `json:"lon"`
	AdditionalProperties []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"additionalProperties"`
}

func (p TflPlace) props() map[string]string {
	m := make(map[string]string, len(p.AdditionalProperties))
	for _, a := range p.AdditionalProperties {
		m[a.Key] = a.Value
	}
	return m
}

func tflInt(s string) int { n, _ := strconv.Atoi(s); return n }

// TflToInformation maps a BikePoint payload to station_information (id, name, lat, lon).
func TflToInformation(raw []byte) (StationInformation, error) {
	var places []TflPlace
	if err := json.Unmarshal(raw, &places); err != nil {
		return StationInformation{}, err
	}
	var si StationInformation
	for _, p := range places {
		si.Data.Stations = append(si.Data.Stations, StationEntity{
			StationID: p.ID,
			Name:      LocalizedText(p.CommonName),
			Lat:       p.Lat,
			Lon:       p.Lon,
		})
	}
	return si, nil
}

// TflToStatus maps a BikePoint payload to station_status. TfL exposes the standard/e-bike split
// (NbStandardBikes + NbEBikes = NbBikes); bikes_disabled is inferred from the dock accounting,
// and the renting/returning/installed flags come from Installed/Locked/Temporary.
func TflToStatus(raw []byte) (StationStatus, error) {
	var places []TflPlace
	if err := json.Unmarshal(raw, &places); err != nil {
		return StationStatus{}, err
	}
	var ss StationStatus
	for _, p := range places {
		m := p.props()
		bikes, ebikes := tflInt(m["NbBikes"]), tflInt(m["NbEBikes"])
		empty, docks := tflInt(m["NbEmptyDocks"]), tflInt(m["NbDocks"])
		disabled := docks - bikes - empty
		if disabled < 0 {
			disabled = 0
		}
		installed := m["Installed"] == "true"
		locked := m["Locked"] == "true"
		temporary := m["Temporary"] == "true"
		flag := func(b bool) Flag {
			if b {
				return 1
			}
			return 0
		}
		ss.Data.Stations = append(ss.Data.Stations, Station{
			StationID:          p.ID,
			NumBikesAvailable:  bikes,
			NumEbikesAvailable: ebikes,
			NumDocksAvailable:  empty,
			NumBikesDisabled:   disabled,
			IsRenting:          flag(installed && !locked && !temporary),
			IsReturning:        flag(!locked),
			IsInstalled:        flag(installed),
		})
	}
	return ss, nil
}
