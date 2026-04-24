package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
)

type FlipkartResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Title string  `json:"title"`
		Brand string  `json:"brand"`
		URL   string  `json:"url"`
		MRP   float64 `json:"mrp"`
		Price float64 `json:"price"`
	} `json:"data"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	pids := []string{
		"RFRHGGAYZTXPZWWQ",
		"RFRH3T3HQQEH6QZM",
		"TVCG4926N3YV6V9S",
		"MOBGHX9W9H2T9GZH",
		"ACCGHY6T7Z7Z7Z7Z",
	}

	var wg sync.WaitGroup
	apiKey := os.Getenv("RAPID_API_KEY")
	pincode := "560066"

	for _, pid := range pids {
		wg.Add(1)
		// Launch each request in a concurrent goroutine
		go func(p string) {
			defer wg.Done()
			fetchAndNotify(p, pincode, apiKey)
		}(pid)
	}

	// Wait for all goroutines to finish before the function exits
	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"completed","processed":%d}`, len(pids))
}

func fetchAndNotify(pid, pincode, apiKey string) {
	url := fmt.Sprintf("https://real-time-flipkart-data2.p.rapidapi.com/product-details?pid=%s&pincode=%s", pid, pincode)
	
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("x-rapidapi-host", "real-time-flipkart-data2.p.rapidapi.com")
	req.Header.Add("x-rapidapi-key", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var result FlipkartResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result.Success {
		sendToDiscord(result.Data.Brand, result.Data.Title, result.Data.Price, result.Data.MRP, result.Data.URL)
	}
}

func sendToDiscord(brand, title string, price, mrp float64, productURL string) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		return
	}

	discount := 0.0
	if mrp > 0 {
		discount = ((mrp - price) / mrp) * 100
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       fmt.Sprintf("🚀 %s Deal Alert!", brand),
				"url":         productURL,
				"description": fmt.Sprintf("✨ **%s**", title),
				"color":       3066993,
				"fields": []map[string]interface{}{
					{"name": "💰 Price", "value": fmt.Sprintf("`₹%.2f`", price), "inline": true},
					{"name": "📉 MRP", "value": fmt.Sprintf("~~₹%.2f~~", mrp), "inline": true},
					{"name": "🎉 Savings", "value": fmt.Sprintf("**%.0f%% OFF**", discount), "inline": true},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(body))
}
