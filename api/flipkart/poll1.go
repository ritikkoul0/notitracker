package handler



import (

	"bytes"

	"encoding/json"

	"fmt"

	"net/http"

	"os"

	"sync"

)



// FlipkartResponse matches your provided JSON exactly

type FlipkartResponse struct {

	Success bool `json:"success"`

	Data    struct {

		Title string  `json:"title"`

		Brand string  `json:"brand"`

		URL   string  `json:"url"`

		MRP   float64 `json:"mrp"`   // Can be int or float in JSON

		Price float64 `json:"price"` // Can be int or float in JSON

	} `json:"data"`

}



func Handler(w http.ResponseWriter, r *http.Request) {

	// List of PIDs to check

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



	for _, pid := range pids {

		wg.Add(1)

		go func(p string) {

			defer wg.Done()

			fetchAndNotify(p, pincode, apiKey)

		}(pid)

	}



	// Wait for all concurrent requests to finish (well within your 30s limit)

	wg.Wait()



	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, `{"status":"success","processed":%d}`, len(pids))

}



func fetchAndNotify(pid, pincode, apiKey string) {

	url := fmt.Sprintf("https://real-time-flipkart-data2.p.rapidapi.com/product-details?pid=%s&pincode=%s", pid, pincode)

	

	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Add("x-rapidapi-host", "real-time-flipkart-data2.p.rapidapi.com")

	req.Header.Add("x-rapidapi-key", apiKey)

	req.Header.Add("Content-Type", "application/json")



	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {

		return

	}

	defer resp.Body.Close()



	var result FlipkartResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {

		return

	}



	if result.Success && result.Data.URL != "" {

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

	if mrp > 0 {

		discount = ((mrp - price) / mrp) * 100

	}



	payload := map[string]interface{}{

		"embeds": []map[string]interface{}{

			{

				"title":       fmt.Sprintf("🚀 %s Alert!", brand),

				"url":         productURL,

				"description": fmt.Sprintf("✨ **%s**", title),

				"color":       3066993, // Green

				"fields": []map[string]interface{}{

					{

						"name":   "💰 Price",

						"value":  fmt.Sprintf("`₹%.2f`", price),

						"inline": true,

					},

					{

						"name":   "📉 MRP",

						"value":  fmt.Sprintf("~~₹%.2f~~", mrp),

						"inline": true,

					},

					{

						"name":   "🎉 Savings",

						"value":  fmt.Sprintf("**%.0f%% OFF**", discount),

						"inline": true,

					},

				},

				"footer": map[string]interface{}{

					"text": "Flipkart Price Bot",

				},

			},

		},

	}



	body, _ := json.Marshal(payload)

	http.Post(webhookURL, "application/json", bytes.NewBuffer(body))

}
