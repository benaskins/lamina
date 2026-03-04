package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const openMeteoMaxBody = 1 << 20 // 1MB

// OpenMeteoClient uses Open-Meteo's free geocoding + weather APIs.
type OpenMeteoClient struct {
	httpClient *http.Client
	geocodeURL string
	weatherURL string
	nowFunc    func() time.Time // for testing; defaults to time.Now
}

// NewOpenMeteoClient creates a weather client using Open-Meteo.
func NewOpenMeteoClient() *OpenMeteoClient {
	return &OpenMeteoClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		geocodeURL: "https://geocoding-api.open-meteo.com/v1/search",
		weatherURL: "https://api.open-meteo.com/v1/forecast",
	}
}

// geoResult is a single geocoding result from Open-Meteo.
type geoResult struct {
	Name      string  `json:"name"`
	Country   string  `json:"country"`
	Admin1    string  `json:"admin1"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// geocodeResponse is the JSON from Open-Meteo's geocoding API.
type geocodeResponse struct {
	Results []geoResult `json:"results"`
}

// parseLocation splits a composite location like "Gosford, Australia" into
// the city name and qualifier parts for filtering geocoding results.
func parseLocation(location string) (string, []string) {
	parts := strings.Split(location, ",")
	city := strings.TrimSpace(parts[0])
	var qualifiers []string
	for _, p := range parts[1:] {
		q := strings.TrimSpace(p)
		if q != "" {
			qualifiers = append(qualifiers, q)
		}
	}
	return city, qualifiers
}

// bestGeoMatch picks the geocoding result that best matches the qualifiers.
// It scores each result by how many qualifiers match its country or admin1 field
// (case-insensitive, substring). Falls back to the first result if no qualifiers match.
func bestGeoMatch(results []geoResult, qualifiers []string) geoResult {
	if len(qualifiers) == 0 || len(results) == 0 {
		return results[0]
	}

	bestIdx := 0
	bestScore := 0
	for i, r := range results {
		score := 0
		for _, q := range qualifiers {
			ql := strings.ToLower(q)
			if strings.Contains(strings.ToLower(r.Country), ql) ||
				strings.Contains(strings.ToLower(r.Admin1), ql) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return results[bestIdx]
}

// weatherAPIResponse is the JSON from Open-Meteo's weather API.
type weatherAPIResponse struct {
	CurrentWeather struct {
		Temperature   float64 `json:"temperature"`
		WindSpeed     float64 `json:"windspeed"`
		WindDirection int     `json:"winddirection"`
		WeatherCode   int     `json:"weathercode"`
		IsDay         int     `json:"is_day"`
	} `json:"current_weather"`
	Hourly struct {
		Time             []string  `json:"time"`
		RelativeHumidity []int     `json:"relative_humidity_2m"`
		ApparentTemp     []float64 `json:"apparent_temperature"`
	} `json:"hourly"`
}

// GetWeather geocodes the location and fetches current weather from Open-Meteo.
func (c *OpenMeteoClient) GetWeather(ctx context.Context, location string) (*WeatherResult, error) {
	// Step 1: Parse location and geocode
	city, qualifiers := parseLocation(location)

	geoURL, err := url.Parse(c.geocodeURL)
	if err != nil {
		return nil, fmt.Errorf("parse geocode URL: %w", err)
	}
	q := geoURL.Query()
	q.Set("name", city)
	q.Set("count", "5")
	q.Set("language", "en")
	q.Set("format", "json")
	geoURL.RawQuery = q.Encode()

	geoReq, err := http.NewRequestWithContext(ctx, http.MethodGet, geoURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create geocode request: %w", err)
	}

	geoResp, err := c.httpClient.Do(geoReq)
	if err != nil {
		return nil, fmt.Errorf("geocode request: %w", err)
	}
	defer geoResp.Body.Close()

	if geoResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocode returned %d", geoResp.StatusCode)
	}

	var geo geocodeResponse
	if err := json.NewDecoder(io.LimitReader(geoResp.Body, openMeteoMaxBody)).Decode(&geo); err != nil {
		return nil, fmt.Errorf("decode geocode: %w", err)
	}

	if len(geo.Results) == 0 {
		return nil, fmt.Errorf("location not found: %s", location)
	}

	loc := bestGeoMatch(geo.Results, qualifiers)

	// Step 2: Fetch weather
	wxURL, err := url.Parse(c.weatherURL)
	if err != nil {
		return nil, fmt.Errorf("parse weather URL: %w", err)
	}
	wq := wxURL.Query()
	wq.Set("latitude", fmt.Sprintf("%.4f", loc.Latitude))
	wq.Set("longitude", fmt.Sprintf("%.4f", loc.Longitude))
	wq.Set("current_weather", "true")
	wq.Set("hourly", "relative_humidity_2m,apparent_temperature")
	wq.Set("forecast_days", "1")
	wxURL.RawQuery = wq.Encode()

	wxReq, err := http.NewRequestWithContext(ctx, http.MethodGet, wxURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create weather request: %w", err)
	}

	wxResp, err := c.httpClient.Do(wxReq)
	if err != nil {
		return nil, fmt.Errorf("weather request: %w", err)
	}
	defer wxResp.Body.Close()

	if wxResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned %d", wxResp.StatusCode)
	}

	var wx weatherAPIResponse
	if err := json.NewDecoder(io.LimitReader(wxResp.Body, openMeteoMaxBody)).Decode(&wx); err != nil {
		return nil, fmt.Errorf("decode weather: %w", err)
	}

	// Build display location
	displayLoc := loc.Name
	if loc.Admin1 != "" {
		displayLoc += ", " + loc.Admin1
	}
	if loc.Country != "" {
		displayLoc += ", " + loc.Country
	}

	// Find the current hour index from the hourly time array.
	hourIdx := currentHourIndex(wx.Hourly.Time, c.now())

	humidity := 0
	if hourIdx < len(wx.Hourly.RelativeHumidity) {
		humidity = wx.Hourly.RelativeHumidity[hourIdx]
	}
	feelsLike := wx.CurrentWeather.Temperature
	if hourIdx < len(wx.Hourly.ApparentTemp) {
		feelsLike = wx.Hourly.ApparentTemp[hourIdx]
	}

	return &WeatherResult{
		Location:    displayLoc,
		Description: weatherCodeToDescription(wx.CurrentWeather.WeatherCode),
		Temperature: wx.CurrentWeather.Temperature,
		FeelsLike:   feelsLike,
		Humidity:    humidity,
		WindSpeed:   wx.CurrentWeather.WindSpeed,
		IsDay:       wx.CurrentWeather.IsDay == 1,
	}, nil
}

// now returns the current time, using nowFunc if set (for testing).
func (c *OpenMeteoClient) now() time.Time {
	if c.nowFunc != nil {
		return c.nowFunc()
	}
	return time.Now()
}

// currentHourIndex finds the index in the hourly time array that best matches
// the current time. Open-Meteo returns times as "2006-01-02T15:04" strings.
// Falls back to 0 if parsing fails or no match is found.
func currentHourIndex(hourlyTimes []string, now time.Time) int {
	nowTrunc := now.Truncate(time.Hour)
	for i, ts := range hourlyTimes {
		t, err := time.Parse("2006-01-02T15:04", ts)
		if err != nil {
			continue
		}
		if !t.Before(nowTrunc) {
			return i
		}
	}
	// If we didn't find a match, fall back to index 0.
	return 0
}

// weatherCodeToDescription maps WMO weather codes to human-readable descriptions.
func weatherCodeToDescription(code int) string {
	switch {
	case code == 0:
		return "Clear sky"
	case code == 1:
		return "Mainly clear"
	case code == 2:
		return "Partly cloudy"
	case code == 3:
		return "Overcast"
	case code >= 45 && code <= 48:
		return "Foggy"
	case code >= 51 && code <= 55:
		return "Drizzle"
	case code >= 56 && code <= 57:
		return "Freezing drizzle"
	case code >= 61 && code <= 65:
		return "Rain"
	case code >= 66 && code <= 67:
		return "Freezing rain"
	case code >= 71 && code <= 75:
		return "Snow"
	case code == 77:
		return "Snow grains"
	case code >= 80 && code <= 82:
		return "Rain showers"
	case code >= 85 && code <= 86:
		return "Snow showers"
	case code == 95:
		return "Thunderstorm"
	case code >= 96 && code <= 99:
		return "Thunderstorm with hail"
	default:
		return "Unknown"
	}
}
