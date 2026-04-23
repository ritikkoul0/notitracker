package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Product struct {
	Name string
	URL  string
}

type ScrapeResult struct {
	Price   string `json:"price"`
	Discord string `json:"discord"`
}

var sharedTransport = &http.Transport{
	MaxIdleConns:    10,
	IdleConnTimeout: 90 * time.Second,
}

func Handler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	productList := []Product{
		{"Havells Rice Cooker", "https://www.flipkart.com/havells-riso-plus-1-8-l-2-bowl-electric-rice-cooker/p/itm9dc31cc3694d7?pid=ECKGZPNF6PSWGBJN"},
		{"LG Microwave Oven", "https://www.flipkart.com/lg-20-l-i-wave-technology-indian-cuisine-auto-cook-menu-steam-clean-anti-bacterial-cavity-health-plus-menu-grill-microwave-oven/p/itmdz5v2y3ckyu9y?pid=MRCDZ5VFHGD7F7UV"},
		{"Atomberg Renesa Fan", "https://www.flipkart.com/atomberg-renesa-halo-smart-voice-controlled-high-air-flow-low-noise-led-speed-indicator-3-year-warranty-bldc-motor-remote-1200-mm-ceiling-fan/p/itm39f1a608fb2aa?pid=FANH9H58ZJ3T5UJM"},
		{"Samsung Bespoke Fridge", "https://www.flipkart.com/samsung-419-l-frost-free-double-door-3-star-convertible-refrigerator-5-in-1-digital-inverter-wifi-enabled-bespoke-ai/p/itm8e086361f0c13?pid=RFRH3T3HQQEH6QZM"},
		{"Whirlpool Chimney", "https://www.flipkart.com/whirlpool-cgbf-pro-903-hac-bk-hood-auto-clean-curved-glass-90-cm-11-years-motor-warranty-heat-autoclean-gesture-control-baffle-filter-powerful-suction-low-noise-wall-mounted-black-1500-cmh-chimney/p/itm12e07fcfaaef4?pid=CHYGT59WPSPNCFG6"},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]ScrapeResult)
	sem := make(chan struct{}, 2)

	for _, p := range productList {
		wg.Add(1)
		go func(prod Product) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			price, mrp, offers, err := scrapeFlipkart(ctx, prod.URL)
			
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				results[prod.Name] = ScrapeResult{Price: err.Error(), Discord: "N/A"}
			} else {
				dErr := sendToDiscord(prod.Name, price, mrp, offers, prod.URL)
				status := "Sent"
				if dErr != nil { status = dErr.Error() }
				results[prod.Name] = ScrapeResult{Price: fmt.Sprintf("₹%.0f", price), Discord: status}
			}
		}(p)
	}

	wg.Wait()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// ... (Paste scrapeFlipkart and sendToDiscord from previous code)
