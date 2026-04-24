package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
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
		"ECKGZPNF6PSWGBJN",
		"MRCDZ5VFHGD7F7UV",
		"FANH9H58ZJ3T5UJM",
		"RFRH3T3HQQEH6QZM",
		"CHYGT59WPSPNCFG6",
	}

	apiKey := os.Getenv("RAPID_API_KEY")
	pincode := "560066"

	var wg sync.WaitGroup
	// Dedicated client with timeout to prevent hanging goroutines
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for _, pid := range pids {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			fetchAndNotify(p, pincode, apiKey, client)
		}(pid)
	}

	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"success","processed":%d}`, len(pids))
}

func fetchAndNotify(pid, pincode, apiKey string, client *http.Client) {
	url := fmt.Sprintf("https://real-time-flipkart-data2.p.rapidapi.com/product-details?pid=%s&pincode=%s", pid, pincode)
	
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("x-rapidapi-host", "real-time-flipkart-data2.p.rapidapi.com")
	req.Header.Add("x-rapidapi-key", apiKey)
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Network error for PID %s: %v\n", pid, err)
		return
	}
	defer resp.Body.Close()

	var result FlipkartResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "Decode error for PID %s: %v\n", pid, err)
		return
	}

	// Use result.Success and verify we actually have price data
	if result.Success && result.Data.Price > 0 {
		sendToDiscord(
			result.Data.Brand,
			result.Data.Title,
			result.Data.Price,
			result.Data.MRP,
			result.Data.URL,
		)
	}
}

func sendToDiscord(brand, title string, price, mrp float64, productURL string) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		return
	}

	discount := 0.0
	if mrp > price && mrp > 0 {
		discount = ((mrp - price) / mrp) * 100
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       fmt.Sprintf("🚀 %s Alert!", brand),
				"url":         productURL,
				"description": fmt.Sprintf("✨ **%s**", title),
				"color":       3066993, 
				"fields": []map[string]interface{}{
					{
						"name":   "💰 Price",
						"value":  fmt.Sprintf("`₹%.0f`", price), // Using %.0f as INR usually doesn't show paise
						"inline": true,
					},
					{
						"name":   "📉 MRP",
						"value":  fmt.Sprintf("~~₹%.0f~~", mrp),
						"inline": true,
					},
					{
						"name":   "🎉 Savings",
						"value":  fmt.Sprintf("**%.0f%% OFF**", discount),
						"inline": true,
					},
				},
				"footer": map[string]interface{}{
					"text": "Flipkart Price Tracker • " + time.Now().Format("15:04"),
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	_, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Discord POST error: %v\n", err)
	}
}
