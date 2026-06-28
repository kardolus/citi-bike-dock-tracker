package types

import (
	"encoding/json"
	"strings"
)

// Flag is a GBFS status flag (is_renting/is_returning/is_installed). Lyft feeds send
// these as 1/0 integers while PBSC (Bicing) and the GBFS spec proper use JSON booleans;
// Flag accepts either and compares == 1 when true. int 1 → 1, true → 1, everything else
// → 0, so NYC (ints) stays byte-identical.
type Flag int

func (f *Flag) UnmarshalJSON(b []byte) error {
	switch strings.TrimSpace(string(b)) {
	case "true":
		*f = 1
	case "false", "null":
		*f = 0
	default:
		var n int
		if err := json.Unmarshal(b, &n); err != nil {
			return err
		}
		*f = Flag(n)
	}
	return nil
}

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
