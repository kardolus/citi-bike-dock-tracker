package types

import "testing"

// TestTflBikePoint covers the London Santander Cycles adapter: TfL's non-GBFS BikePoint shape
// (string key/value additionalProperties, standard/e-bike split, lock/install flags) must map
// onto the internal types. Dock accounting: disabled = NbDocks - NbBikes - NbEmptyDocks.
func TestTflBikePoint(t *testing.T) {
	raw := []byte(`[
	  {"id":"BikePoints_1","commonName":"River Street, Clerkenwell","lat":51.5292,"lon":-0.1100,
	   "additionalProperties":[
	     {"key":"NbBikes","value":"3"},{"key":"NbStandardBikes","value":"0"},{"key":"NbEBikes","value":"3"},
	     {"key":"NbEmptyDocks","value":"15"},{"key":"NbDocks","value":"19"},
	     {"key":"Locked","value":"false"},{"key":"Installed","value":"true"},{"key":"Temporary","value":"false"}]},
	  {"id":"BikePoints_2","commonName":"Phillimore Gardens, Kensington","lat":51.5,"lon":-0.2,
	   "additionalProperties":[
	     {"key":"NbBikes","value":"10"},{"key":"NbEBikes","value":"2"},
	     {"key":"NbEmptyDocks","value":"20"},{"key":"NbDocks","value":"32"},
	     {"key":"Locked","value":"true"},{"key":"Installed","value":"true"},{"key":"Temporary","value":"false"}]}
	]`)

	si, err := TflToInformation(raw)
	if err != nil {
		t.Fatalf("TflToInformation: %v", err)
	}
	if len(si.Data.Stations) != 2 || si.Data.Stations[0].StationID != "BikePoints_1" ||
		si.Data.Stations[0].Name.String() != "River Street, Clerkenwell" {
		t.Fatalf("info mapping wrong: %+v", si.Data.Stations)
	}

	ss, err := TflToStatus(raw)
	if err != nil {
		t.Fatalf("TflToStatus: %v", err)
	}
	a := ss.Data.Stations[0]
	if a.Bikes() != 3 || a.NumEbikesAvailable != 3 || a.NumDocksAvailable != 15 ||
		a.Disabled() != 1 || a.IsRenting != 1 || a.IsInstalled != 1 {
		t.Errorf("station A: bikes=%d ebikes=%d docks=%d disabled=%d renting=%d installed=%d",
			a.Bikes(), a.NumEbikesAvailable, a.NumDocksAvailable, a.Disabled(), a.IsRenting, a.IsInstalled)
	}
	// Locked station: not renting, not returning; disabled = 32-10-20 = 2.
	b := ss.Data.Stations[1]
	if b.IsRenting != 0 || b.IsReturning != 0 || b.Disabled() != 2 {
		t.Errorf("station B (locked): renting=%d returning=%d disabled=%d", b.IsRenting, b.IsReturning, b.Disabled())
	}
}
