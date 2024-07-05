package utils

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const MaxSegments = 10

type SegmentStatus struct {
	URL       string  `json:"URL"`
	Duration  float64 `json:"Duration"`
	LoadTime  float64 `json:"LoadTime"`
	IsDelayed bool    `json:"IsDelayed"`
}

func MeasureSegmentLoadTime(url string) float64 {
	start := time.Now()
	_, err := http.Get(url)
	if err != nil {
		fmt.Println("Error loading segment:", err)
	}
	return time.Since(start).Seconds()
}

func ResolveURL(baseURL, relURL string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		fmt.Println("Error parsing base URL:", err)
		return relURL
	}

	ref, err := url.Parse(relURL)
	if err != nil {
		fmt.Println("Error parsing relative URL:", err)
		return relURL
	}

	return base.ResolveReference(ref).String()
}
