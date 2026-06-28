package types

import (
	"bytes"
	"encoding/json"
	"strings"
)

// LocalizedText is a name that arrives as a plain string (GBFS v2 — Lyft/Smovengo/PBSC v2)
// or a localized array [{"text":…,"language":…}] (GBFS v3 — PBSC v3, e.g. BA Ecobici).
// String() returns the Spanish entry when present, else the first; for a plain string it's
// just that string, so v2 feeds (incl. NYC) stay byte-identical.
type LocalizedText string

func (t *LocalizedText) String() string { return string(*t) }

func (t *LocalizedText) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*t = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*t = LocalizedText(s)
		return nil
	}
	var arr []struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	if len(arr) == 0 {
		*t = ""
		return nil
	}
	pick := arr[0].Text
	for _, e := range arr {
		if e.Language == "es" {
			pick = e.Text
			break
		}
	}
	*t = LocalizedText(pick)
	return nil
}

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
