package geocode

import (
	"groupie-tracker/internal/models"
	"log"
	"sync"
	"time"
)

// retryDelay is a var so tests do not have to wait 2 seconds.
var retryDelay = 2 * time.Second

// geocodeWithRetry: one retry after retryDelay, then we give up.
func geocodeWithRetry(location string) (models.GeoResult, error) {
	result, err := Geocode(location)
	if err != nil {
		time.Sleep(retryDelay)
		return Geocode(location)
	}
	return result, nil
}

// GeocodeAll resolves all locations with 3 workers pulling from a channel.
// It checks the cache first and only goes to the network on a miss.
// Locations that fail are skipped (log and carry on).
func GeocodeAll(locations []string, c *Cache) map[string]models.GeoResult {
	results := make(map[string]models.GeoResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	queue := make(chan string, len(locations))
	for _, loc := range locations {
		queue <- loc
	}
	close(queue)

	const workers = 1
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for loc := range queue {
				if cached, ok := c.Get(loc); ok {
					mu.Lock()
					results[loc] = cached
					mu.Unlock()
					continue
				}
				result, err := geocodeWithRetry(loc)
				if err != nil {
					log.Printf("geocode failed for %q: %v", loc, err)
					continue
				}
				c.Set(loc, result)
				mu.Lock()
				results[loc] = result
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if err := c.Save(); err != nil {
		log.Printf("cannot save geocache: %v", err)
	}
	return results
}
