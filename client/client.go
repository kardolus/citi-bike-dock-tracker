package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kardolus/citi-bike-dock-tracker/http"
	"github.com/kardolus/citi-bike-dock-tracker/types"
	"time"
)

const (
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
	serviceURL   string
}

type ClientBuilder struct {
	caller       http.Caller
	stationMap   map[string]types.StationEntity
	timeProvider TimeProvider
	serviceURL   string
	filteredIDs  map[string]bool
}

func NewClientBuilder(caller http.Caller, timeProvider TimeProvider) *ClientBuilder {
	return &ClientBuilder{
		caller:       caller,
		stationMap:   make(map[string]types.StationEntity),
		timeProvider: timeProvider,
		serviceURL:   DefaultServiceURL,
		filteredIDs:  make(map[string]bool),
	}
}

// WithIDFilter adds a filter to only look for specific station IDs
func (b *ClientBuilder) WithIDFilter(ids []string) *ClientBuilder {
	for _, id := range ids {
		b.filteredIDs[id] = true
	}
	return b
}

// WithServiceURL overwrites the default service URL
func (b *ClientBuilder) WithServiceURL(url string) *ClientBuilder {
	b.serviceURL = url
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
		var (
			item        types.NormalizedStation
			stationInfo types.StationEntity
		)

		if _, ok := c.stationMap[stationStatus.StationID]; ok {
			stationInfo = c.stationMap[stationStatus.StationID]
		} else {
			continue
		}

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

		result.Stations = append(result.Stations, item)
	}

	result.TimeStamp = c.timeProvider.Now()

	return result, nil
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
