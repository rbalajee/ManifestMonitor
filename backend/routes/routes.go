package routes

import (
	"ManifestMonitor/backend/middleware"
	"ManifestMonitor/backend/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/grafov/m3u8"
	"github.com/zencoder/go-dash/mpd"
)

var sessions = make(map[string]chan bool)
var sessionData = make(map[string][]utils.SegmentStatus)
var fetchedSegments = make(map[string]map[string]bool) // To track fetched segments
var mu sync.Mutex

func SetupRoutes() {
	frontendPath, err := filepath.Abs("./frontend")
	if err != nil {
		fmt.Println("Error getting absolute path:", err)
		return
	}

	fs := http.FileServer(http.Dir(frontendPath))
	http.Handle("/", middleware.Recoverer(cacheControlMiddleware(fs)))
	http.HandleFunc("/monitor", monitorHandler)
	http.HandleFunc("/startMonitoring", startMonitoringHandler)
	http.HandleFunc("/stopMonitoring", stopMonitoringHandler) // New handler for stopping monitoring
}

func cacheControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func startMonitoringHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
		ID  string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if !isValidURL(req.URL) {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	mu.Lock()
	if _, exists := sessions[req.ID]; exists {
		close(sessions[req.ID])
	}
	stopChan := make(chan bool)
	sessions[req.ID] = stopChan
	sessionData[req.ID] = []utils.SegmentStatus{}
	fetchedSegments[req.ID] = make(map[string]bool) // Initialize fetched segments tracker for the session
	mu.Unlock()

	go monitorManifest(req.URL, stopChan, req.ID)

	fmt.Fprintf(w, "Monitoring started for %s", req.URL)
}

func stopMonitoringHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	mu.Lock()
	if stopChan, exists := sessions[req.ID]; exists {
		close(stopChan)
		delete(sessions, req.ID)
		delete(sessionData, req.ID)
		delete(fetchedSegments, req.ID) // Properly clean up the session's fetched segments map
		fmt.Printf("Stopped monitoring for session %s\n", req.ID)
	}
	mu.Unlock()
}

func isValidURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func monitorManifest(url string, stopChan chan bool, sessionID string) {
	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			FetchAndParseManifest(url, sessionID)
		case <-stopChan:
			return
		}
	}
}

func FetchAndParseManifest(url string, sessionID string) {
	if strings.HasSuffix(url, ".m3u8") {
		fetchAndParseHLSManifest(url, sessionID)
	} else if strings.HasSuffix(url, ".mpd") {
		fetchAndParseMPDManifest(url, sessionID)
	} else {
		fmt.Println("Unsupported manifest type")
	}
}

func fetchAndParseHLSManifest(url string, sessionID string) {
	fmt.Println("Fetching HLS manifest from URL:", url)
	response, err := http.Get(url)
	if err != nil {
		fmt.Println("Error fetching HLS manifest:", err)
		return
	}
	defer response.Body.Close()

	playlist, listType, err := m3u8.DecodeFrom(response.Body, true)
	if err != nil {
		fmt.Println("Error decoding HLS manifest:", err)
		return
	}

	switch listType {
	case m3u8.MEDIA:
		mediaPlaylist := playlist.(*m3u8.MediaPlaylist)
		fmt.Printf("Parsed media playlist with %d segments\n", len(mediaPlaylist.Segments))
		parseMediaPlaylist(mediaPlaylist, url, sessionID)
	case m3u8.MASTER:
		masterPlaylist := playlist.(*m3u8.MasterPlaylist)
		fmt.Printf("Parsed master playlist with %d variants\n", len(masterPlaylist.Variants))
		if len(masterPlaylist.Variants) > 0 {
			chunklistURL := utils.ResolveURL(url, masterPlaylist.Variants[0].URI)
			fmt.Println("Fetching chunklist from URL:", chunklistURL)
			FetchAndParseChunklist(chunklistURL, sessionID)
		}
	default:
		fmt.Println("Unknown HLS playlist type")
	}
}

func fetchAndParseMPDManifest(url string, sessionID string) {
	fmt.Println("Fetching MPD manifest from URL:", url)
	response, err := http.Get(url)
	if err != nil {
		fmt.Println("Error fetching MPD manifest:", err)
		return
	}
	defer response.Body.Close()

	mpdManifest, err := mpd.Read(response.Body)
	if err != nil {
		fmt.Println("Error decoding MPD manifest:", err)
		return
	}

	fmt.Printf("Parsed MPD manifest with %d periods\n", len(mpdManifest.Periods))
	for _, period := range mpdManifest.Periods {
		for _, adaptationSet := range period.AdaptationSets {
			for _, representation := range adaptationSet.Representations {
				baseURL := url
				if representation.BaseURL != nil {
					baseURL = *representation.BaseURL
				}

				if representation.SegmentTemplate != nil {
					template := representation.SegmentTemplate
					if template.Media != nil && template.Initialization != nil {
						media := *template.Media
						initialization := *template.Initialization

						// Construct initialization segment URL
						initSegmentURL := utils.ResolveURL(baseURL, initialization)
						fmt.Println("Initialization Segment URL:", initSegmentURL)

						for i := 0; i < 10; i++ { // Example: Fetch first 10 segments
							segmentURL := utils.ResolveURL(baseURL, strings.Replace(media, "$Number$", fmt.Sprintf("%d", i+1), -1))
							mu.Lock()
							if fetchedSegments[sessionID] == nil {
								fetchedSegments[sessionID] = make(map[string]bool)
							}
							if !fetchedSegments[sessionID][segmentURL] { // Check if the segment has been fetched
								newSegment := utils.SegmentStatus{
									URL:       segmentURL,
									Duration:  0, // Calculate duration if needed
									LoadTime:  0,
									IsDelayed: false,
								}
								if !containsSegment(sessionData[sessionID], newSegment) {
									sessionData[sessionID] = append(sessionData[sessionID], newSegment)
									if len(sessionData[sessionID]) > utils.MaxSegments {
										sessionData[sessionID] = sessionData[sessionID][len(sessionData[sessionID])-utils.MaxSegments:]
									}
								}
								fetchedSegments[sessionID][segmentURL] = true // Mark segment as fetched
							}
							mu.Unlock()
						}
					}
				}
			}
		}
	}
	fmt.Printf("Updated MonitoringResults with %d segments\n", len(sessionData[sessionID]))
}

func FetchAndParseChunklist(url string, sessionID string) {
	response, err := http.Get(url)
	if err != nil {
		fmt.Println("Error fetching chunklist:", err)
		return
	}
	defer response.Body.Close()

	playlist, listType, err := m3u8.DecodeFrom(response.Body, true)
	if err != nil {
		fmt.Println("Error decoding chunklist:", err)
		return
	}

	if listType == m3u8.MEDIA {
		mediaPlaylist := playlist.(*m3u8.MediaPlaylist)
		fmt.Printf("Parsed media playlist with %d segments\n", len(mediaPlaylist.Segments))
		parseMediaPlaylist(mediaPlaylist, url, sessionID)
	} else {
		fmt.Println("Chunklist is not a media playlist")
	}
}

func parseMediaPlaylist(pl *m3u8.MediaPlaylist, baseURL string, sessionID string) {
	newResults := []utils.SegmentStatus{}

	for _, segment := range pl.Segments {
		if segment != nil {
			segmentURL := utils.ResolveURL(baseURL, segment.URI)
			loadTime := utils.MeasureSegmentLoadTime(segmentURL)
			isDelayed := loadTime > 2.0

			newSegment := utils.SegmentStatus{
				URL:       segmentURL,
				Duration:  segment.Duration,
				LoadTime:  loadTime,
				IsDelayed: isDelayed,
			}

			mu.Lock()
			if fetchedSegments[sessionID] == nil {
				fetchedSegments[sessionID] = make(map[string]bool)
			}
			if !fetchedSegments[sessionID][segmentURL] { // Check if the segment has been fetched
				if !containsSegment(sessionData[sessionID], newSegment) {
					newResults = append(newResults, newSegment)
				}
				fetchedSegments[sessionID][segmentURL] = true // Mark segment as fetched
			}
			mu.Unlock()
		}
	}

	if len(newResults) > 0 {
		mu.Lock()
		sessionData[sessionID] = append(sessionData[sessionID], newResults...)
		if len(sessionData[sessionID]) > utils.MaxSegments {
			sessionData[sessionID] = sessionData[sessionID][len(sessionData[sessionID])-utils.MaxSegments:]
		}
		mu.Unlock()
	}

	fmt.Printf("Updated MonitoringResults with %d segments\n", len(sessionData[sessionID]))
}

func containsSegment(results []utils.SegmentStatus, segment utils.SegmentStatus) bool {
	for _, existingSegment := range results {
		if existingSegment.URL == segment.URL {
			return true
		}
	}
	return false
}

func monitorHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("id")
	mu.Lock()
	data, exists := sessionData[sessionID]
	mu.Unlock()
	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
