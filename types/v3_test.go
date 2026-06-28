package types

import (
	"encoding/json"
	"testing"
)

// TestGBFSv3Parsing covers the PBSC GBFS v3 shape (e.g. BA Ecobici): a localized `name`
// array and num_vehicles_* in place of num_bikes_*. v2 (Lyft/NYC) must keep working —
// see also the golden tests in client/.
func TestGBFSv3Parsing(t *testing.T) {
	// v3 also makes short_name a localized array and last_updated/last_reported RFC3339 strings.
	info := `{"last_updated":"2026-06-28T00:00:00Z","ttl":60,"data":{"stations":[{"station_id":"2","lat":-34.5,"lon":-58.3,
	  "name":[{"text":"002 - Retiro I","language":"en"},{"text":"002 - Retiro","language":"es"}],
	  "short_name":[{"text":"002","language":"es"}]}]}}`
	var si StationInformation
	if err := json.Unmarshal([]byte(info), &si); err != nil {
		t.Fatalf("v3 station_information: %v", err)
	}
	if got := si.Data.Stations[0].Name.String(); got != "002 - Retiro" {
		t.Errorf("v3 name: want Spanish entry %q, got %q", "002 - Retiro", got)
	}

	status := `{"last_updated":"2026-06-28T00:00:00Z","data":{"stations":[{"station_id":"2","num_vehicles_available":23,
	  "num_vehicles_disabled":3,"num_docks_available":15,"is_renting":true,"is_installed":true,
	  "last_reported":"2026-06-28T00:00:00Z"}]}}`
	var ss StationStatus
	if err := json.Unmarshal([]byte(status), &ss); err != nil {
		t.Fatalf("v3 station_status: %v", err)
	}
	s := ss.Data.Stations[0]
	if s.Bikes() != 23 || s.Disabled() != 3 || s.IsRenting != 1 {
		t.Errorf("v3 status: bikes=%d disabled=%d renting=%d (want 23/3/1)", s.Bikes(), s.Disabled(), s.IsRenting)
	}

	// v2 (plain string name + num_bikes_available + int flag) stays intact.
	var si2 StationInformation
	_ = json.Unmarshal([]byte(`{"data":{"stations":[{"station_id":"a","name":"Plain"}]}}`), &si2)
	if si2.Data.Stations[0].Name.String() != "Plain" {
		t.Errorf("v2 string name regressed: %q", si2.Data.Stations[0].Name.String())
	}
	var ss2 StationStatus
	_ = json.Unmarshal([]byte(`{"data":{"stations":[{"station_id":"a","num_bikes_available":7,"is_renting":1}]}}`), &ss2)
	if ss2.Data.Stations[0].Bikes() != 7 || ss2.Data.Stations[0].IsRenting != 1 {
		t.Errorf("v2 status regressed")
	}
}
