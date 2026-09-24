package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func writeGPXFromEncodedPolyline(w http.ResponseWriter, trkName string, routeJSON []byte) error {
	// Expecting a single Route JSON with optional legs.polyline.encodedPolyline
	var route struct {
		Duration string `json:"duration"`
		Polyline struct {
			EncodedPolyline string `json:"encodedPolyline"`
		} `json:"polyline"`
		Legs []struct {
			Duration string `json:"duration"`
			Polyline struct {
				EncodedPolyline string `json:"encodedPolyline"`
			} `json:"polyline"`
		} `json:"legs"`
	}
	if err := json.Unmarshal(routeJSON, &route); err != nil {
		return fmt.Errorf("failed to parse route json: %w", err)
	}

	segments := make([]GPXTrackSegment, 0, len(route.Legs))
	now := time.Now()

	buildSeg := func(t time.Time, duration int, encoded string) (GPXTrackSegment, error) {
		coords, err := decodePolyline(encoded)
		if err != nil {
			return GPXTrackSegment{}, fmt.Errorf("failed to decode polyline: %w", err)
		}
		pts := make([]GPXPoint, 0, len(coords))
		// Interval between points, so that timestamps are evenly spaced.
		// Duration in seconds is divided by the number of points minus 1 to get the interval between points.
		interval := time.Duration(float64(duration) / float64(len(coords)-1) * float64(time.Second))
		for _, c := range coords {
			lat := c[0]
			lon := c[1]
			timeStr := t.UTC().Format("2006-01-02T15:04:05.00Z")
			pts = append(pts, GPXPoint{Lat: lat, Lon: lon, Time: timeStr})
			t = t.Add(interval)
		}
		return GPXTrackSegment{Points: pts}, nil
	}

	if len(route.Legs) > 0 {
		for _, leg := range route.Legs {
			if leg.Polyline.EncodedPolyline == "" {
				continue
			}
			duration, err := strconv.Atoi(strings.TrimSuffix(leg.Duration, "s"))
			seg, err := buildSeg(now, duration, leg.Polyline.EncodedPolyline)
			now = now.Add(time.Duration(duration) * time.Second)
			if err != nil {
				return err
			}
			segments = append(segments, seg)
		}
	} else if route.Polyline.EncodedPolyline != "" {
		// Fallback to whole-route polyline when legs are unavailable.
		duration, err := strconv.Atoi(strings.TrimSuffix(route.Duration, "s"))
		seg, err := buildSeg(now, duration, route.Polyline.EncodedPolyline)
		if err != nil {
			return err
		}
		segments = append(segments, seg)
	} else {
		return fmt.Errorf("route contains no polyline data")
	}

	gpx := GPX{
		Version: "1.1",
		Creator: "Google Maps Routes API based router app",
		Xmlns:   "https://www.topografix.com/GPX/1/1",
		Trk: GPXTrack{
			Name:     trkName,
			Segments: segments,
		},
	}

	data, err := xml.MarshalIndent(gpx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal GPX: %w", err)
	}

	w.Header().Set("Content-Type", "application/gpx+xml; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=route.gpx")
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(data)
	return nil
}

// HTTP handler: /route/{mode}/{token}?point=lat,lng&point=lat,lng[&point=...]&heading=deg
func routeHandler(w http.ResponseWriter, r *http.Request) {
	// Parse path and get path values and query parameters
	mode := r.PathValue("mode")
	token := r.PathValue("token")

	queryValues := r.URL.Query()
	points := queryValues["point"]
	heading := queryValues.Get("heading")

	apiKey := os.Getenv("GOOGLE_MAPS_API_KEY")

	// Authenticating token
	if token != os.Getenv("ROUTER_APP_AUTH_TOKEN") {
		http.Error(w, "Invalid Auth Token", http.StatusUnauthorized)
		return
	}

	if mode == "" {
		http.Error(w, "missing mode in path", http.StatusBadRequest)
		return
	}

	if token == "" {
		http.Error(w, "missing token in path", http.StatusBadRequest)
		return
	}

	// Query params
	if len(points) < 2 {
		http.Error(w, "provide at least two 'point' parameters like point=lat,lng", http.StatusBadRequest)
		return
	}

	// Validate and transform points
	latlngs := make([]latLng, 0, len(points))
	for _, p := range points {
		la, lo, err := parseLatLng(p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		latlngs = append(latlngs, latLng{Latitude: la, Longitude: lo})
	}

	// Heading (optional, degrees 0..360). Applied to the origin waypoint.
	var headingPtr *int
	if h := strings.TrimSpace(heading); h != "" {
		hv, err := strconv.Atoi(h)
		if err != nil || hv < 0 || hv > 360 {
			http.Error(w, "invalid heading: must be an integer 0..360", http.StatusBadRequest)
			return
		}
		headingPtr = &hv
	}

	// Build computeRoutes request body
	// origin and destination from first and last points, intermediates are those in-between
	origin := waypoint{
		Location: &struct {
			LatLng  latLng `json:"latLng"`
			Heading *int   `json:"heading,omitempty"`
		}{LatLng: latlngs[0], Heading: headingPtr},
	}
	destination := waypoint{
		Location: &struct {
			LatLng  latLng `json:"latLng"`
			Heading *int   `json:"heading,omitempty"`
		}{LatLng: latlngs[len(latlngs)-1]},
	}
	var intermediates []waypoint
	if len(latlngs) > 2 {
		intermediates = make([]waypoint, 0, len(latlngs)-2)
		for _, ll := range latlngs[1 : len(latlngs)-1] {
			wp := waypoint{
				Location: &struct {
					LatLng  latLng `json:"latLng"`
					Heading *int   `json:"heading,omitempty"`
				}{LatLng: ll},
			}
			intermediates = append(intermediates, wp)
		}
	}

	reqBody := computeRoutesRequest{
		Origin:            origin,
		Destination:       destination,
		Intermediates:     intermediates,
		TravelMode:        travelMode(mode),
		RoutingPreference: "TRAFFIC_AWARE_OPTIMAL",
		PolylineEncoding:  "ENCODED_POLYLINE",
		PolyLineQuality:   "HIGH_QUALITY",
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to marshal request: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare HTTP request to Routes API v2
	// Endpoint: https://routes.googleapis.com/directions/v2:computeRoutes
	req, err := http.NewRequest(http.MethodPost, "https://routes.googleapis.com/directions/v2:computeRoutes", bytes.NewReader(bodyBytes))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	// Provide API key and field mask via headers as per Routes API guidance
	req.Header.Set("X-Goog-Api-Key", apiKey)
	// Request only the encoded polyline to minimize payload
	req.Header.Set("X-Goog-FieldMask", "routes.duration,routes.polyline.encodedPolyline,routes.legs.duration,routes.legs.polyline.encodedPolyline")

	// Execute request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to call Routes API: %v", err), http.StatusBadGateway)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("Routes API error: status %d body: %s", resp.StatusCode, string(b)), http.StatusBadGateway)
		return
	}

	var cr computeRoutesResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode Routes API response: %v", err), http.StatusBadGateway)
		return
	}

	if len(cr.Routes) == 0 {
		http.Error(w, "no route found", http.StatusNotFound)
		return
	}

	// Convert to GPX and return
	trackName := fmt.Sprintf("Route (%s)", strings.ToUpper(travelMode(mode)))
	routeBytes, err := json.Marshal(cr.Routes[0])

	if err != nil {
		http.Error(w, fmt.Sprintf("failed to marshal route json: %v", err), http.StatusInternalServerError)
		return
	}
	if err := writeGPXFromEncodedPolyline(w, trackName, routeBytes); err != nil {
		http.Error(w, fmt.Sprintf("failed to write GPX: %v", err), http.StatusInternalServerError)
		return
	}
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /route/{mode}/{token}", routeHandler)

	addr := ":8080"
	log.Printf("Listening on %s ...", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
