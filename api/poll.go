package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type FlipkartResponse struct {
	Data struct {
		SellingPrice float64 `json:"selling_price"`
		MRP          float64 `json:"mrp"`
	} `json:"data"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	// 1. Fetch Price
	url := "https://flipkart-apis.p.rapidapi.com/backend/rapidapi/product-details?pid=RFRH3T3HQQEH6QZM"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("x-rapidapi-host", "flipkart-apis.p.rapidapi.com")
	req.Header.Add("x-rapidapi-key", os.Getenv("RAPID_API_KEY"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to reach Flipkart", 500)
		return
	}
	defer resp.Body.Close()

	var result FlipkartResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		http.Error(w, "JSON Decode Error", 500)
		return
	}

	// 2. Notify Discord
	sendToDiscord(result.Data.SellingPrice, result.Data.MRP)

	// 3. Response for Vercel Logs
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"notified","price":%v}`, result.Data.SellingPrice)
}

func sendToDiscord(price, mrp float64) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		return
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title": "🛒 Price Poller: Samsung Refrigerator",
				"color": 3066993, // Green
				"fields": []map[string]interface{}{
					{"name": "Selling Price", "value": fmt.Sprintf("₹%.2f", price), "inline": true},
					{"name": "MRP", "value": fmt.Sprintf("₹%.2f", mrp), "inline": true},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(body))
}
