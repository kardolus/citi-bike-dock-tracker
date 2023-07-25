package client

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kardolus/citi-bike-dock-tracker/http"
	"github.com/kardolus/citi-bike-dock-tracker/types"
	"os"
	"time"
)

const (
	DefaultInterval        = 60 // in seconds
	DefaultServiceURL      = "https://gbfs.citibikenyc.com"
	ErrEmptyResponse       = "empty response"
	GoogleMapsQuery        = "https://www.google.com/maps/?q=%f,%f"
	StationInformationPath = "/gbfs/en/station_information.json"
	StationStatusPath      = "/gbfs/en/station_status.json"
)

type TimeProvider interface {
	Now() time.Time
}

type RealTime struct{}

func (RealTime) Now() time.Time {
	return time.Now()
}

// Ensure RealTime implements TimeProvider interface
var _ TimeProvider = &RealTime{}

type Client struct {
	caller       http.Caller
	stationMap   map[string]types.StationEntity
	timeProvider TimeProvider
	interval     int
	serviceURL   string
}

type ClientBuilder struct {
	caller       http.Caller
	stationMap   map[string]types.StationEntity
	timeProvider TimeProvider
	interval     int
	serviceURL   string
	filteredIDs  map[string]bool
}

func NewClientBuilder() *ClientBuilder {
	return &ClientBuilder{
		caller:       http.New(),
		stationMap:   make(map[string]types.StationEntity),
		interval:     DefaultInterval,
		timeProvider: RealTime{},
		serviceURL:   DefaultServiceURL,
		filteredIDs:  make(map[string]bool),
	}
}

// WithCaller overwrites the default http caller
func (b *ClientBuilder) WithCaller(caller http.Caller) *ClientBuilder {
	b.caller = caller
	return b
}

// WithIDFilter adds a filter to only look for specific station IDs
func (b *ClientBuilder) WithIDFilter(ids []string) *ClientBuilder {
	for _, id := range ids {
		b.filteredIDs[id] = true
	}
	return b
}

// WithInterval overwrites the default interval
func (b *ClientBuilder) WithInterval(interval int) *ClientBuilder {
	b.interval = interval
	return b
}

// WithServiceURL overwrites the default service URL
func (b *ClientBuilder) WithServiceURL(url string) *ClientBuilder {
	b.serviceURL = url
	return b
}

// WithTimeProvider overwrites the default time provider
func (b *ClientBuilder) WithTimeProvider(provider TimeProvider) *ClientBuilder {
	b.timeProvider = provider
	return b
}

// Build creates the Client instance
func (b *ClientBuilder) Build() (*Client, error) {
	stationInfo, err := b.getStationInformation()
	if err != nil {
		return nil, err
	}

	for _, station := range stationInfo.Data.Stations {
		if len(b.filteredIDs) > 0 {
			if _, ok := b.filteredIDs[station.StationID]; ok {
				b.stationMap[station.StationID] = station
			}
		} else {
			b.stationMap[station.StationID] = station
		}
	}

	return &Client{
		caller:       b.caller,
		stationMap:   b.stationMap,
		interval:     b.interval,
		timeProvider: b.timeProvider,
		serviceURL:   b.serviceURL,
	}, nil
}

func (b *ClientBuilder) getStationInformation() (types.StationInformation, error) {
	raw, err := b.caller.Get(b.serviceURL + StationInformationPath)
	if err != nil {
		return types.StationInformation{}, err
	}

	var response types.StationInformation
	if err := processResponse(raw, &response); err != nil {
		return types.StationInformation{}, err
	}

	return response, nil
}

// ParseStationData fetches station status information from the Citi Bike API and combines
// it with pre-fetched station information to create a set of normalized data.
//
// The normalized data consists of details such as station ID, name, status, capacity, and
// the number of available bikes, e-bikes, docks, and scooters, as well as operational status flags.
//
// The function returns a NormalizedStationData instance containing the collected information,
// or an error if fetching or processing the data fails. Note that the function only includes
// stations which have corresponding entries in the pre-fetched station information data.
func (c *Client) ParseStationData() (types.NormalizedStationData, error) {
	var result types.NormalizedStationData

	statusData, err := c.getStationStatus()
	if err != nil {
		return types.NormalizedStationData{}, err
	}

	for _, stationStatus := range statusData.Data.Stations {
		if stationInfo, ok := c.stationMap[stationStatus.StationID]; ok {
			item := normalizeStationData(stationStatus, stationInfo)
			result.Stations = append(result.Stations, item)
		}
	}

	result.TimeStamp = c.timeProvider.Now()

	return result, nil
}

// PrintStationDataJSONL fetches station status information from the Citi Bike API and combines
// it with pre-fetched station information to create a set of normalized data. The normalized data
// is printed to stdout in the JSONL format.
//
// The function runs indefinitely, fetching new data every minute. To stop the function, you must
// interrupt the program manually.
func (c *Client) PrintStationDataJSONL() {
	for {
		stationData, err := c.gatherStationData()
		if err != nil {
			continue
		}

		for _, data := range stationData {
			jsonl, err := json.Marshal(data)
			if err != nil {
				continue
			}
			fmt.Println(string(jsonl))
		}

		time.Sleep(time.Duration(c.interval) * time.Second)
	}
}

// PrintStationDataCSV gathers station data periodically according to the client's interval
// and prints it to the standard output (stdout) in CSV format. The CSV data includes a header row,
// and each subsequent row represents the current state of a station.
// The fields are StationID, Name, Longitude, Latitude, Location, Status, BikesAvailable,
// EBikesAvailable, BikesDisabled, DocksAvailable, DocksDisabled, IsReturning, IsRenting,
// IsInstalled, and TimeStamp. In case of an error while gathering data, the function continues with
// the next iteration after the sleep interval. If writing to the CSV writer fails, the function logs
// the error and exits. The function runs indefinitely, and each iteration is separated by a sleep
// interval defined by the client.
func (c *Client) PrintStationDataCSV() {
	w := csv.NewWriter(os.Stdout)

	_ = w.Write([]string{
		"StationID",
		"Name",
		"Longitude",
		"Latitude",
		"Location",
		"Status",
		"BikesAvailable",
		"EBikesAvailable",
		"BikesDisabled",
		"DocksAvailable",
		"DocksDisabled",
		"IsReturning",
		"IsRenting",
		"IsInstalled",
		"TimeStamp",
	})

	for {
		stationData, err := c.gatherStationData()
		if err != nil {
			continue
		}

		for _, data := range stationData {
			record := []string{
				data.Station.ID,
				data.Station.Name,
				fmt.Sprint(data.Station.Longitude),
				fmt.Sprint(data.Station.Latitude),
				data.Station.Location,
				data.Station.Status,
				fmt.Sprint(data.Station.BikesAvailable),
				fmt.Sprint(data.Station.EBikesAvailable),
				fmt.Sprint(data.Station.BikesDisabled),
				fmt.Sprint(data.Station.DocksAvailable),
				fmt.Sprint(data.Station.DocksDisabled),
				fmt.Sprint(data.Station.IsReturning),
				fmt.Sprint(data.Station.IsRenting),
				fmt.Sprint(data.Station.IsInstalled),
				data.TimeStamp.Format(time.RFC3339),
			}
			_ = w.Write(record)
		}
		w.Flush()

		time.Sleep(time.Duration(c.interval) * time.Second)
	}
}

func (c *Client) gatherStationData() ([]types.NormalizedStationDataTS, error) {
	var stationData []types.NormalizedStationDataTS

	statusData, err := c.getStationStatus()
	if err != nil {
		return nil, err
	}

	for _, stationStatus := range statusData.Data.Stations {
		if stationInfo, ok := c.stationMap[stationStatus.StationID]; ok {
			item := normalizeStationData(stationStatus, stationInfo)
			data := types.NormalizedStationDataTS{
				Station:   item,
				TimeStamp: c.timeProvider.Now(),
			}
			stationData = append(stationData, data)
		}
	}

	return stationData, nil
}

func (c *Client) getStationStatus() (types.StationStatus, error) {
	raw, err := c.caller.Get(c.serviceURL + StationStatusPath)
	if err != nil {
		return types.StationStatus{}, err
	}

	var response types.StationStatus
	if err := processResponse(raw, &response); err != nil {
		return types.StationStatus{}, err
	}

	return response, nil
}

func processResponse(raw []byte, v interface{}) error {
	if raw == nil {
		return errors.New(ErrEmptyResponse)
	}

	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

func normalizeStationData(stationStatus types.Station, stationInfo types.StationEntity) types.NormalizedStation {
	var item types.NormalizedStation

	item.ID = stationStatus.StationID
	item.Name = stationInfo.Name
	item.Longitude = stationInfo.Lon
	item.Latitude = stationInfo.Lat
	item.Location = fmt.Sprintf(GoogleMapsQuery, stationInfo.Lat, stationInfo.Lon)
	item.Status = stationStatus.StationStatus
	item.BikesAvailable = stationStatus.NumBikesAvailable
	item.EBikesAvailable = stationStatus.NumEbikesAvailable
	item.BikesDisabled = stationStatus.NumBikesDisabled
	item.DocksAvailable = stationStatus.NumDocksAvailable
	item.DocksDisabled = stationStatus.NumDocksDisabled
	item.ScootersAvailable = stationStatus.NumScootersAvailable
	item.ScootersUnavailable = stationStatus.NumScootersUnavailable
	item.IsReturning = stationStatus.IsReturning == 1
	item.IsRenting = stationStatus.IsRenting == 1
	item.IsInstalled = stationStatus.IsInstalled == 1

	return item
}
