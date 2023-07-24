# Citi Bike Dock Tracker

## Table of Contents

1. [Introduction](#introduction)
    - [Output](#output)
2. [Installation](#installation)
   - [Apple M1 chips](#apple-m1-chips)
   - [macOS Intel chips](#macos-intel-chips)
   - [Linux (amd64)](#linux-amd64)
   - [Linux (arm64)](#linux-arm64)
   - [Windows (amd64)](#windows-amd64)
3. [Usage](#usage)
    - [Filtering by ID](#filtering-by-id)
4. [Development](#development)
5. [Uninstallation](#uninstallation)
6. [Contributing](#contributing)

## Introduction

This repository is designed to fetch, store, and analyze time series data related to the status of CitiBike docks in New
York City. The information stored includes data about open docks, the number of standard and electric bikes available,
and changes over time.

The primary data source for this project is CitiBike's General Bikeshare Feed Specification (GBFS) data feeds, which
provide real-time information about station status (`https://gbfs.citibikenyc.com/gbfs/en/station_status.json`) and
station information (`https://gbfs.citibikenyc.com/gbfs/en/station_information.json`).

The `station_status.json` file is used to gather real-time information about each dock station's status, including the
number of available bikes, open docks, and other relevant data. The `station_information.json` file is used to map the
station IDs to their respective human-readable names, providing easier data interpretation.

### Output

When you run the dockscan-cli, it produces a JSON output for each Citi Bike station. Here's an example of what one of
these JSON objects might look like:

```json
{
  "id": "5faf99b8-9046-450f-9d2a-d13279b3d016",
  "name": "Hoboken Ave at Monmouth St",
  "longitude": -74.04696375131607,
  "latitude": 40.73520838045357,
  "location": "https://www.google.com/maps/?q=40.735208,-74.046964",
  "status": "active",
  "bikesAvailable": 21,
  "eBikesAvailable": 7,
  "bikesDisabled": 4,
  "docksAvailable": 7,
  "docksDisabled": 0,
  "isReturning": true,
  "isRenting": true,
  "isInstalled": true
}
```

The output provides valuable information such as the station's name, its location (both in terms of longitude and
latitude and a Google Maps link), the status of the station, and detailed statistics about the number of available bikes
and docks.

## Installation

### Apple M1 chips

```shell
curl -L -o dockscan https://github.com/kardolus/citi-bike-dock-tracker/releases/download/v1.0.0/dockscan-darwin-arm64 && chmod +x dockscan && sudo mv dockscan /usr/local/bin/
```

### macOS Intel chips

```shell
curl -L -o dockscan https://github.com/kardolus/citi-bike-dock-tracker/releases/download/v1.0.0/dockscan-darwin-amd64 && chmod +x dockscan && sudo mv dockscan /usr/local/bin/
```

### Linux (amd64)

```shell
curl -L -o dockscan https://github.com/kardolus/citi-bike-dock-tracker/releases/download/v1.0.0/dockscan-linux-amd64 && chmod +x dockscan && sudo mv dockscan /usr/local/bin/
```

### Linux (arm64)

```shell
curl -L -o dockscan https://github.com/kardolus/citi-bike-dock-tracker/releases/download/v1.0.0/dockscan-linux-arm64 && chmod +x dockscan && sudo mv dockscan /usr/local/bin/
```

### Windows (amd64)

Download the binary
from [this link](https://github.com/kardolus/citi-bike-dock-tracker/releases/download/v1.0.0/dockscan-windows-amd64.exe)
and add it to your PATH.

Choose the appropriate command for your system, which will download the binary, make it executable, and move it to your
/usr/local/bin directory (or %PATH% on Windows) for easy access.

## Usage

To use the dockscan, follow these steps:

1. Clone the repository.
2. Ensure that you have Golang set up on your local machine.
3. Run the Golang scripts to start fetching and storing the data using the `dockscan` binary, like so:

```shell
./bin/dockscan info
```

This will fetch the current data and output it to your terminal in JSON format.

To better interpret the JSON output, you can use a tool like `jq`:

```shell
./bin/dockscan info | jq .
```

Instructions on how to run the Python analysis scripts on Noteable will be added soon.

### Filtering by ID

You can filter the data to only show the status of certain stations by providing their IDs with the `--id` flag:

```shell
./bin/dockscan info --id 37a37e5b-f975-4f92-a897-dca8e4670631 --id c00ef46d-fcde-48e2-afbd-0fb595fe3fa7
```

## Development

Follow these steps for running tests and building the application:

1. Run the tests using the following scripts:

For unit tests, run:

```shell
./scripts/unit.sh
```

For integration tests, run:

```shell
./scripts/integration.sh
```

For contract tests, run:

```shell
./scripts/contract.sh
```

To run all tests, use:

```shell
./scripts/all-tests.sh
```

2. Build the app using the installation script:

```shell
./scripts/install.sh
```

3. After a successful build, test the application with the following command:

```shell
./bin/dockscan -h
```

## Uninstallation

If for any reason you wish to uninstall the dockscan CLI application from your system, you can do so by following these
steps:

### MacOS / Linux

If you installed the binary directly, remove it as such:

```shell
sudo rm /usr/local/bin/dockscan
```

### Windows

1. Navigate to the location of the `dockscan` binary in your system, which should be in your PATH.

2. Delete the `dockscan` binary.

## Contributing

We appreciate contributions to the dockscan-cli. Please feel free to submit issues and pull requests.
