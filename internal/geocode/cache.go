package geocode

import (
	"encoding/json"
	"groupie-tracker/internal/models"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type Cache struct {
	path string
	data map[string]models.GeoResult
	mu   sync.RWMutex
}

func LoadCache(path string) *Cache {
	c := &Cache{
		path: path,
		data: make(map[string]models.GeoResult),
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("no geocache found at %s, starting fresh", path)
		} else {
			log.Printf("cannot read geocache at %s, starting fresh: %v", path, err)
		}
		return c
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(&c.data)
	if err != nil {
		os.Remove(c.path)
		c.data = make(map[string]models.GeoResult)
		return c
	}
	return c
}

func (c *Cache) Save() error {
	dir := filepath.Dir(c.path)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	tmpPath := c.path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer file.Close()

	c.mu.RLock()
	defer c.mu.RUnlock()

	err = json.NewEncoder(file).Encode(c.data)
	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	err = file.Close()
	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, c.path)
}

func (c *Cache) Get(location string) (models.GeoResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	coord, ok := c.data[location]
	return coord, ok
}

func (c *Cache) Set(location string, coord models.GeoResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[location] = coord
}
