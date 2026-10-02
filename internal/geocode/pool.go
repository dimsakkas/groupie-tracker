package geocode

import (
	"groupie-tracker/internal/models"
	"log"
	"sync"
	"time"
)

func geocodeWithRetry(location string) (models.GeoResult, error) {
	coordinates, err := Geocode(location)
	if err != nil {
		time.Sleep(2 * time.Second)
		coordinates, err = Geocode(location)
		return coordinates, err
	}
	return coordinates, nil
}

func GeocodeAll(locations []string, c *Cache) map[string]models.GeoResult {
	results := make(map[string]models.GeoResult)
	var mu sync.Mutex
	var wg sync.WaitGroup
	queue := make(chan string, len(locations))
	for _, loc := range locations {
		queue <- loc
	}
	close(queue)
	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func() {
			defer wg.Done()
			for loc := range queue {
				coords, ok := c.Get(loc)
				if ok {
					mu.Lock()
					results[loc] = coords
					mu.Unlock()
					continue
				}
				coords, err := geocodeWithRetry(loc)
				if err != nil {
					log.Printf("geocode failed for %q: %v\n", loc, err)
					continue
				}
				c.Set(loc, coords)
				mu.Lock()
				results[loc] = coords
				mu.Unlock()
			}

		}()
	}
	wg.Wait()
	return results
}
