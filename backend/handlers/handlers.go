package handlers

import (
	"ManifestMonitor/backend/utils"
	"encoding/json"
	"fmt"
	"net/http"
)

var MonitoringResults []utils.SegmentStatus = []utils.SegmentStatus{}

func MonitorHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if MonitoringResults == nil {
		fmt.Println("MonitoringResults is nil")
	} else {
		fmt.Printf("Returning %d results\n", len(MonitoringResults))
	}
	json.NewEncoder(w).Encode(MonitoringResults)
}
