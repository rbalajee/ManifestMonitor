package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/grafov/m3u8"
)

// SegmentStatus holds the details for each segment being monitored
type SegmentStatus struct {
	URL       string  `json:"url"`
	Duration  float64 `json:"duration"`
	LoadTime  float64 `json:"load_time"`
	IsDelayed bool    `json:"isDelayed"`
}

// manifestURL is the URL of the m3u8 manifest to be monitored
var manifestURL = "https://bcovlive-a.akamaihd.net/r77df0dffb8c44094b029f1f924124818/us-west-2/6057994532001/playlist.m3u8"

// monitoringResults will store the monitoring data
var monitoringResults []SegmentStatus

func main() {
	// Clear the monitoringResults at startup
	monitoringResults = []SegmentStatus{}
	log.Println("Cleared monitoring results at startup")

	// Serve static files from the frontend directory
	fs := http.FileServer(http.Dir("./frontend"))
	http.Handle("/", cacheControlMiddleware(fs))

	// Set up the HTTP handler for the "/monitor" endpoint
	http.HandleFunc("/monitor", monitorHandler)

	// Start the monitoring in a separate goroutine
	go monitorManifest()

	// Start the HTTP server
	log.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// monitorHandler responds to HTTP requests with the monitoring results
func monitorHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store") // Ensure no caching
	//log.Printf("Returning monitoring results: %v\n", monitoringResults) // Log the monitoring results
	json.NewEncoder(w).Encode(monitoringResults)
}

// cacheControlMiddleware is a middleware to add Cache-Control headers to responses
func cacheControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		next.ServeHTTP(w, r)
	})
}

// monitorManifest continuously fetches and parses the m3u8 manifest
func monitorManifest() {
	for {
		log.Println("Starting new monitoring cycle, clearing previous results")
		// Initialize monitoringResults to an empty slice at the start of each cycle
		monitoringResults = []SegmentStatus{}

		results, err := fetchAndParseManifest(manifestURL)
		if err != nil {
			log.Printf("Error fetching or parsing manifest: %v", err)
			continue
		}
		//log.Printf("Fetched and parsed manifest: %d segments\n", len(results)) // Log the number of segments fetched
		monitoringResults = results
		time.Sleep(6 * time.Second) // Adjust the interval to 6 seconds
	}
}

// fetchAndParseManifest fetches the m3u8 manifest and parses the segments
func fetchAndParseManifest(manifestURL string) ([]SegmentStatus, error) {
	resp, err := http.Get(manifestURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	base, err := url.Parse(manifestURL)
	if err != nil {
		return nil, err
	}

	p, listType, err := m3u8.DecodeFrom(resp.Body, true)
	if err != nil {
		return nil, err
	}

	log.Printf("List type: %v", listType)
	var results []SegmentStatus

	switch listType {
	case m3u8.MEDIA:
		results, err = parseMediaPlaylist(base, p.(*m3u8.MediaPlaylist))
		if err != nil {
			log.Printf("Error parsing media playlist: %v", err)
		}
	case m3u8.MASTER:
		masterPlaylist := p.(*m3u8.MasterPlaylist)
		if len(masterPlaylist.Variants) > 0 {
			// Pick the first variant
			variantURL, err := resolveURL(base, masterPlaylist.Variants[0].URI)
			if err != nil {
				log.Printf("Error resolving variant URL %s: %v", masterPlaylist.Variants[0].URI, err)
				return nil, err
			}
			results, err = fetchAndParseManifest(variantURL)
			if err != nil {
				log.Printf("Error fetching and parsing variant manifest: %v", err)
			}
		} else {
			log.Println("No variants found in master playlist")
		}
	}

	log.Printf("Total segments found: %d", len(results))
	return results, nil
}

// parseMediaPlaylist parses the segments from a media playlist
func parseMediaPlaylist(base *url.URL, mediaPlaylist *m3u8.MediaPlaylist) ([]SegmentStatus, error) {
	var results []SegmentStatus
	for i, segment := range mediaPlaylist.Segments {
		if segment == nil {
			continue
		}
		segURL, err := resolveURL(base, segment.URI)
		if err != nil {
			log.Printf("Error resolving segment URL %s: %v", segment.URI, err)
			continue
		}
		start := time.Now()
		resp, err := http.Get(segURL)
		if err != nil {
			log.Printf("Error fetching segment URL %s: %v", segURL, err)
			continue
		}
		resp.Body.Close()
		loadTime := time.Since(start).Seconds()

		results = append(results, SegmentStatus{
			URL:       segURL,
			Duration:  segment.Duration,
			LoadTime:  loadTime,
			IsDelayed: loadTime > 1, // Example threshold; adjust as needed
		})
		log.Printf("Segment %d: URL=%s, Duration=%.2f, LoadTime=%.2f, IsDelayed=%v",
			i, segURL, segment.Duration, loadTime, loadTime > 1)
	}
	return results, nil
}

// resolveURL resolves relative URLs against a base URL
func resolveURL(base *url.URL, rel string) (string, error) {
	u, err := url.Parse(rel)
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(u)
	return resolved.String(), nil
}
