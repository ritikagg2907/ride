package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	kafkapkg "github.com/ride-hailing/shared/pkg/kafka"
	redispkg "github.com/ride-hailing/shared/pkg/redis"
	"github.com/rs/zerolog/log"
)

// We use a simplified geo-cell approach: encode lat/lng to ~5km² grid cells
// without the h3-go CGO dependency. Cell key = "cell:{lat2}:{lng2}" (rounded to 0.05°).
// This gives ~5km precision and avoids CGO in alpine builds.

const (
	geoKey         = "drivers:geo"
	cellPrecision  = 0.05 // ~5.5km grid
	surgeKeyPrefix = "surge:cell:"
	demandPrefix   = "demand:"
	overridePrefix = "surge:override:"
	surgeKeyLegacy = "surge:multiplier" // backward compat with trip-service direct read
)

type SurgeService struct {
	redis    *redispkg.Client
	producer *kafkapkg.Producer
}

func New(rc *redispkg.Client, producer *kafkapkg.Producer) *SurgeService {
	return &SurgeService{redis: rc, producer: producer}
}

// cellKey encodes lat/lng into a grid cell identifier.
func cellKey(lat, lng float64) string {
	clat := math.Round(lat/cellPrecision) * cellPrecision
	clng := math.Round(lng/cellPrecision) * cellPrecision
	return fmt.Sprintf("%.2f:%.2f", clat, clng)
}

// IncrementDemand records a ride request in the demand counter for its cell.
func (s *SurgeService) IncrementDemand(ctx context.Context, lat, lng float64) {
	cell := cellKey(lat, lng)
	key := demandPrefix + cell
	_, _ = s.redis.Incr(ctx, key, 60*time.Second)
}

// Recompute recalculates surge for a set of cells (or all active cells).
func (s *SurgeService) Recompute(ctx context.Context, cells []string) {
	if len(cells) == 0 {
		// Discover active cells from demand keys
		keys, _ := s.redis.Scan(ctx, demandPrefix+"*")
		for _, k := range keys {
			cells = append(cells, strings.TrimPrefix(k, demandPrefix))
		}
	}

	for _, cell := range cells {
		mult := s.computeMultiplier(ctx, cell)
		redisKey := surgeKeyPrefix + cell
		_ = s.redis.SetRaw(ctx, redisKey, fmt.Sprintf("%.2f", mult), 90*time.Second)
		// Also update the legacy global key used by trip-service
		_ = s.redis.SetRaw(ctx, surgeKeyLegacy, fmt.Sprintf("%.2f", mult), 90*time.Second)
	}
}

func (s *SurgeService) computeMultiplier(ctx context.Context, cell string) float64 {
	// Check admin override first
	if override, err := s.redis.GetRaw(ctx, overridePrefix+cell); err == nil {
		if f, err := strconv.ParseFloat(override, 64); err == nil {
			return f
		}
	}

	// Count demand (ride requests in last 60s)
	demandStr, _ := s.redis.GetRaw(ctx, demandPrefix+cell)
	demand, _ := strconv.ParseFloat(demandStr, 64)

	// Count supply: available drivers near this cell center
	parts := strings.Split(cell, ":")
	if len(parts) != 2 {
		return 1.0
	}
	lat, _ := strconv.ParseFloat(parts[0], 64)
	lng, _ := strconv.ParseFloat(parts[1], 64)
	locs, err := s.redis.GeoRadius(ctx, geoKey, lng, lat, cellPrecision*111.0, 100)
	if err != nil {
		return 1.0
	}
	supply := 0
	for _, loc := range locs {
		meta, _ := s.redis.HGetAll(ctx, "driver:"+loc.Name+":meta")
		if meta["status"] == "available" {
			supply++
		}
	}

	ratio := demand / math.Max(float64(supply), 1)
	mult := 1.0 + (ratio-1)*0.5
	return math.Min(math.Max(mult, 1.0), 5.0)
}

// GetMultiplier reads cached surge for a lat/lng point.
func (s *SurgeService) GetMultiplier(ctx context.Context, lat, lng float64) float64 {
	cell := cellKey(lat, lng)
	v, err := s.redis.GetRaw(ctx, surgeKeyPrefix+cell)
	if err != nil {
		return 1.0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 1.0
	}
	return f
}

// SetOverride sets an admin override for a specific cell.
func (s *SurgeService) SetOverride(ctx context.Context, lat, lng, mult float64, ttl time.Duration) {
	cell := cellKey(lat, lng)
	_ = s.redis.SetRaw(ctx, overridePrefix+cell, fmt.Sprintf("%.2f", mult), ttl)
}

// AllCells returns a snapshot of all active cell multipliers.
func (s *SurgeService) AllCells(ctx context.Context) map[string]float64 {
	keys, _ := s.redis.Scan(ctx, surgeKeyPrefix+"*")
	result := make(map[string]float64, len(keys))
	for _, k := range keys {
		v, _ := s.redis.GetRaw(ctx, k)
		f, _ := strconv.ParseFloat(v, 64)
		cell := strings.TrimPrefix(k, surgeKeyPrefix)
		result[cell] = f
	}
	return result
}

// RunTicker triggers periodic recompute every 30s.
func (s *SurgeService) RunTicker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Debug().Msg("surge: periodic recompute")
			s.Recompute(ctx, nil)
		}
	}
}
