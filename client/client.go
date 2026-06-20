package client

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kardolus/citi-bike-dock-tracker/http"
	"github.com/kardolus/citi-bike-dock-tracker/metrics"
	"github.com/kardolus/citi-bike-dock-tracker/types"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"
)

// BBox is a geographic bounding box used to filter stations by location.
type BBox struct {
	MinLat, MinLon, MaxLat, MaxLon float64
}

func (b BBox) contains(lat, lon float64) bool {
	return lat >= b.MinLat && lat <= b.MaxLat && lon >= b.MinLon && lon <= b.MaxLon
}

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
	location, _ := time.LoadLocation("America/New_York")
	return time.Now().In(location)
}

// Ensure RealTime implements TimeProvider interface
var _ TimeProvider = &RealTime{}

type Client struct {
	caller          http.Caller
	stationMap      map[string]types.StationEntity
	timeProvider    TimeProvider
	interval        int
	serviceURL      string
	currentDate     time.Time
	outputDirectory string
}

type ClientBuilder struct {
	caller          http.Caller
	stationMap      map[string]types.StationEntity
	timeProvider    TimeProvider
	interval        int
	serviceURL      string
	filteredIDs     map[string]bool
	bbox            *BBox
	outputDirectory string
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

// WithBBox restricts stations to those within a geographic bounding box
func (b *ClientBuilder) WithBBox(box BBox) *ClientBuilder {
	b.bbox = &box
	return b
}

// WithInterval overwrites the default interval
func (b *ClientBuilder) WithInterval(interval int) *ClientBuilder {
	b.interval = interval
	return b
}

// WithOutputDirectory specifies the directory to which CSV files should be written
func (b *ClientBuilder) WithOutputDirectory(dir string) *ClientBuilder {
	b.outputDirectory = dir
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
		// include unless an ID filter or bbox excludes it (both applied when set)
		if len(b.filteredIDs) > 0 {
			if _, ok := b.filteredIDs[station.StationID]; !ok {
				continue
			}
		}
		if b.bbox != nil && !b.bbox.contains(station.Lat, station.Lon) {
			continue
		}
		b.stationMap[station.StationID] = station
	}

	return &Client{
		caller:          b.caller,
		stationMap:      b.stationMap,
		interval:        b.interval,
		timeProvider:    b.timeProvider,
		serviceURL:      b.serviceURL,
		currentDate:     startOfDay(b.timeProvider.Now()),
		outputDirectory: b.outputDirectory,
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
func (c *Client) PrintStationDataCSV(excludeColumns []string) {
	var w *csv.Writer

	if c.outputDirectory == "" {
		w = csv.NewWriter(os.Stdout)
	} else {
		w = createNewWriter(c.currentDate, c.outputDirectory)
	}

	headers := []string{
		"ID",
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
	}

	// Prepare headers
	var finalHeaders []string
	for _, h := range headers {
		if !contains(excludeColumns, h) {
			finalHeaders = append(finalHeaders, h)
		}
	}

	_ = w.Write(finalHeaders)

	for {
		currentDay := startOfDay(c.timeProvider.Now())
		if currentDay.After(c.currentDate) {
			w.Flush()
			w = createNewWriter(currentDay, c.outputDirectory)
			_ = w.Write(finalHeaders)
			c.currentDate = currentDay
		}

		stationData, err := c.gatherStationData()
		if err != nil {
			continue
		}

		for _, data := range stationData {
			var record []string
			if !contains(excludeColumns, "ID") {
				record = append(record, data.Station.ID)
			}
			if !contains(excludeColumns, "Name") {
				record = append(record, data.Station.Name)
			}
			if !contains(excludeColumns, "Longitude") {
				record = append(record, fmt.Sprint(data.Station.Longitude))
			}
			if !contains(excludeColumns, "Latitude") {
				record = append(record, fmt.Sprint(data.Station.Latitude))
			}
			if !contains(excludeColumns, "Location") {
				record = append(record, data.Station.Location)
			}
			if !contains(excludeColumns, "BikesAvailable") {
				record = append(record, fmt.Sprint(data.Station.BikesAvailable))
			}
			if !contains(excludeColumns, "EBikesAvailable") {
				record = append(record, fmt.Sprint(data.Station.EBikesAvailable))
			}
			if !contains(excludeColumns, "BikesDisabled") {
				record = append(record, fmt.Sprint(data.Station.BikesDisabled))
			}
			if !contains(excludeColumns, "DocksAvailable") {
				record = append(record, fmt.Sprint(data.Station.DocksAvailable))
			}
			if !contains(excludeColumns, "DocksDisabled") {
				record = append(record, fmt.Sprint(data.Station.DocksDisabled))
			}
			if !contains(excludeColumns, "IsReturning") {
				record = append(record, fmt.Sprint(data.Station.IsReturning))
			}
			if !contains(excludeColumns, "IsRenting") {
				record = append(record, fmt.Sprint(data.Station.IsRenting))
			}
			if !contains(excludeColumns, "IsInstalled") {
				record = append(record, fmt.Sprint(data.Station.IsInstalled))
			}
			if !contains(excludeColumns, "TimeStamp") {
				record = append(record, data.TimeStamp.Format(time.RFC3339))
			}
			_ = w.Write(record)
		}
		w.Flush()

		time.Sleep(time.Duration(c.interval) * time.Second)
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func (c *Client) gatherStationData() ([]types.NormalizedStationDataTS, error) {
	var stationData []types.NormalizedStationDataTS

	statusData, err := c.getStationStatus()
	if err != nil {
		return nil, err
	}

	now := c.timeProvider.Now()

	for _, stationStatus := range statusData.Data.Stations {
		if stationInfo, ok := c.stationMap[stationStatus.StationID]; ok {
			item := normalizeStationData(stationStatus, stationInfo)
			data := types.NormalizedStationDataTS{
				Station:   item,
				TimeStamp: now,
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

func createNewWriter(currentDay time.Time, dir string) *csv.Writer {
	filename := filepath.Join(dir, currentDay.Format("2006-01-02")+".csv")
	file, _ := os.Create(filename)

	return csv.NewWriter(file)
}

func normalizeStationData(stationStatus types.Station, stationInfo types.StationEntity) types.NormalizedStation {
	var item types.NormalizedStation

	item.ID = stationStatus.StationID
	item.Name = stationInfo.Name
	item.Longitude = stationInfo.Lon
	item.Latitude = stationInfo.Lat
	item.Location = fmt.Sprintf(GoogleMapsQuery, stationInfo.Lat, stationInfo.Lon)
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

func processResponse(raw []byte, v interface{}) error {
	if raw == nil {
		return errors.New(ErrEmptyResponse)
	}

	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

const createDockStatusTable = `
CREATE TABLE IF NOT EXISTS dock_status (
    station_id           text        NOT NULL,
    name                 text        NOT NULL,
    longitude            double precision,
    latitude             double precision,
    bikes_available      integer,
    ebikes_available     integer,
    bikes_disabled       integer,
    docks_available      integer,
    docks_disabled       integer,
    scooters_available   integer,
    scooters_unavailable integer,
    is_returning         boolean,
    is_renting           boolean,
    is_installed         boolean,
    ts                   timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS dock_status_station_ts_idx ON dock_status (station_id, ts);
CREATE INDEX IF NOT EXISTS dock_status_ts_idx ON dock_status (ts);
`

// IngestPostgres runs the polling loop, writing each tracked station's status to
// the dock_status table on every interval. It creates the table if missing and
// runs indefinitely. Health is surfaced via the metrics package.
func (c *Client) IngestPostgres(dsn string) error {
	if dsn == "" {
		return errors.New("postgres DSN is empty (set DATABASE_URL)")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(2)

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	if _, err := db.Exec(createDockStatusTable); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	log.Printf("ingesting to postgres every %ds (%d stations tracked)", c.interval, len(c.stationMap))

	for {
		metrics.IncPolls()
		stationData, err := c.gatherStationData()
		if err != nil {
			metrics.IncFetchError()
			log.Printf("fetch error: %v", err)
			time.Sleep(time.Duration(c.interval) * time.Second)
			continue
		}
		if err := c.insertBatch(db, stationData); err != nil {
			metrics.IncDBError()
			log.Printf("db write error: %v", err)
		} else {
			metrics.AddRows(len(stationData))
			metrics.SetStations(len(stationData))
			metrics.MarkSuccess(c.timeProvider.Now())
		}
		time.Sleep(time.Duration(c.interval) * time.Second)
	}
}

func (c *Client) insertBatch(db *sql.DB, data []types.NormalizedStationDataTS) error {
	if len(data) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO dock_status
        (station_id,name,longitude,latitude,bikes_available,ebikes_available,bikes_disabled,
         docks_available,docks_disabled,scooters_available,scooters_unavailable,
         is_returning,is_renting,is_installed,ts)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, d := range data {
		s := d.Station
		if _, err := stmt.Exec(s.ID, s.Name, s.Longitude, s.Latitude, s.BikesAvailable,
			s.EBikesAvailable, s.BikesDisabled, s.DocksAvailable, s.DocksDisabled,
			s.ScootersAvailable, s.ScootersUnavailable, s.IsReturning, s.IsRenting,
			s.IsInstalled, d.TimeStamp); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
