package main

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// GPX types
type GPX struct {
	XMLName xml.Name `xml:"gpx"`
	Version string   `xml:"version,attr"`
	Creator string   `xml:"creator,attr"`
	Xmlns   string   `xml:"xmlns,attr"`
	Trk     GPXTrack `xml:"trk"`
}

type GPXTrack struct {
	Name     string            `xml:"name,omitempty"`
	Segments []GPXTrackSegment `xml:"trkseg"`
}

type GPXTrackSegment struct {
	Points []GPXPoint `xml:"trkpt"`
}

type GPXPoint struct {
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Time string  `xml:"time,omitempty"`
}

// Routes API v2 request/response (minimal shapes)
type latLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type waypoint struct {
	Location *struct {
		LatLng  latLng `json:"latLng"`
		Heading *int   `json:"heading,omitempty"`
	} `json:"location,omitempty"`
	// Other fields like via, sideOfRoad, etc., omitted for brevity
}

// Data structure for Google Maps Directions API v2 request body
type computeRoutesRequest struct {
	Origin            waypoint   `json:"origin"`
	Destination       waypoint   `json:"destination"`
	Intermediates     []waypoint `json:"intermediates,omitempty"`
	TravelMode        string     `json:"travelMode,omitempty"`
	RoutingPreference string     `json:"routingPreference,omitempty"`
	PolylineEncoding  string     `json:"polylineEncoding,omitempty"`
	PolyLineQuality   string     `json:"polylineQuality,omitempty"`
	DepartureTime     string     `json:"departureTime,omitempty"`
}

// Data structure for Google Maps Directions API v2 response body
type computeRoutesResponse struct {
	Routes []struct {
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
	} `json:"routes"`
}

// Polyline decoding (Google encoded polyline algorithm)
func decodePolyline(encoded string) ([][2]float64, error) {
	var points [][2]float64
	index, lat, lng := 0, 0, 0

	for index < len(encoded) {
		// Decode latitude
		var result, shift int
		for {
			if index >= len(encoded) {
				return nil, fmt.Errorf("unexpected end of encoded polyline while decoding latitude")
			}
			b := int(encoded[index] - 63)
			index++
			result |= (b & 0x1F) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		dlat := (result >> 1) ^ (-(result & 1))
		lat += dlat

		// Decode longitude
		result, shift = 0, 0
		for {
			if index >= len(encoded) {
				return nil, fmt.Errorf("unexpected end of encoded polyline while decoding longitude")
			}
			b := int(encoded[index] - 63)
			index++
			result |= (b & 0x1F) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		dlng := (result >> 1) ^ (-(result & 1))
		lng += dlng

		points = append(points, [2]float64{float64(lat) / 1e5, float64(lng) / 1e5})
	}

	return points, nil
}

// Helper to parse "lat,lng" into two floats
func parseLatLng(s string) (float64, float64, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid point format, expected 'lat,lng': %s", s)
	}
	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid latitude in point '%s': %w", s, err)
	}
	lng, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid longitude in point '%s': %w", s, err)
	}
	return lat, lng, nil
}

// Map simple string modes to Routes API v2 RouteTravelMode
// Valid values include: "DRIVE", "BICYCLE", "WALK", "TWO_WHEELER", "TRANSIT" (availability varies).
func travelMode(mode string) string {
	switch strings.ToLower(mode) {
	case "driving", "drive", "car":
		return "DRIVE"
	case "walking", "walk":
		return "WALK"
	case "bicycling", "bicycle", "bike":
		return "BICYCLE"
	case "two_wheeler", "scooter", "motorcycle":
		return "TWO_WHEELER"
	case "transit":
		return "TRANSIT"
	default:
		return "DRIVE"
	}
}
