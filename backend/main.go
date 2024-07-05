package main

import (
	"ManifestMonitor/backend/routes"
	"fmt"
	"net/http"
)

/*
var (
	manifestURL string
	mu          sync.Mutex
)
*/

func main() {
	routes.SetupRoutes()
	//go monitorManifest()

	//http.HandleFunc("/startMonitoring", startMonitoringHandler)

	fmt.Println("Server is running at http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Error starting server:", err)
	}
}

/*

func startMonitoringHandler(w http.ResponseWriter, r *http.Request) {
	var request struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mu.Lock()
	manifestURL = request.URL
	mu.Unlock()

	fmt.Println("Received manifest URL:", manifestURL)
	w.WriteHeader(http.StatusOK)
}



func monitorManifest() {
	for {
		mu.Lock()
		url := manifestURL
		mu.Unlock()

		if url != "" {
			routes.FetchAndParseManifest(url)
		}
		time.Sleep(6 * time.Second)
	}
}

*/
