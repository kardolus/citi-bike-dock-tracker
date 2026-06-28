package types

import (
	"encoding/json"
	"strings"
)

// coerceID normalizes a GBFS station_id: Lyft feeds (Citi Bike, Capital Bikeshare,
// Ecobici) send it as a JSON string, while Smovengo (Vélib') sends it as a JSON number.
// We always store it as a string.
func coerceID(raw json.RawMessage) string {
	return strings.Trim(strings.TrimSpace(string(raw)), `"`)
}

// UnmarshalJSON lets StationEntity.station_id be either a string or a number.
func (s *StationEntity) UnmarshalJSON(b []byte) error {
	type alias StationEntity
	aux := struct {
		StationID json.RawMessage `json:"station_id"`
		*alias
	}{alias: (*alias)(s)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	s.StationID = coerceID(aux.StationID)
	return nil
}

// UnmarshalJSON lets Station.station_id be either a string or a number.
func (s *Station) UnmarshalJSON(b []byte) error {
	type alias Station
	aux := struct {
		StationID json.RawMessage `json:"station_id"`
		*alias
	}{alias: (*alias)(s)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	s.StationID = coerceID(aux.StationID)
	return nil
}
