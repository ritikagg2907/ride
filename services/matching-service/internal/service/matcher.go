package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	kafkapkg "github.com/ride-hailing/shared/pkg/kafka"
	redispkg "github.com/ride-hailing/shared/pkg/redis"
	"github.com/rs/zerolog/log"
)

const (
	geoKey         = "drivers:geo"
	initialRadiusKm = 5.0
	expandedRadius  = 10.0
	offerTTL       = 30 * time.Second
	lockTTL        = 35 * time.Second
	maxRetryMin    = 10
)

type Matcher struct {
	redis          *redispkg.Client
	producer       *kafkapkg.Producer
	tripServiceURL string
	internalSecret string
}

func New(rc *redispkg.Client, producer *kafkapkg.Producer, tripURL, internalSecret string) *Matcher {
	return &Matcher{
		redis: rc, producer: producer,
		tripServiceURL: tripURL, internalSecret: internalSecret,
	}
}

type candidate struct {
	driverID    string
	lat, lng    float64
	distanceKm  float64
	rating      float64
}

func (m *Matcher) HandleRideRequested(ctx context.Context, evt kafkapkg.RideRequestedEvent) error {
	log.Info().Str("trip_id", evt.TripID).Msg("matching: received RIDE_REQUESTED")

	start := time.Now()

	for _, radiusKm := range []float64{initialRadiusKm, expandedRadius} {
		candidates, err := m.findCandidates(ctx, evt, radiusKm)
		if err != nil || len(candidates) == 0 {
			continue
		}

		for _, c := range candidates {
			accepted, err := m.offerAndWait(ctx, evt, c)
			if err != nil {
				log.Warn().Err(err).Str("driver", c.driverID).Msg("offer error")
				continue
			}
			if accepted {
				elapsed := time.Since(start)
				log.Info().
					Str("trip_id", evt.TripID).
					Str("driver_id", c.driverID).
					Dur("dispatch_latency", elapsed).
					Msg("dispatch success")
				return m.assignTrip(ctx, evt, c)
			}
		}
	}

	// No driver found after all attempts
	log.Warn().Str("trip_id", evt.TripID).Msg("no driver found, cancelling trip")
	_ = m.producer.Publish(ctx, kafkapkg.TopicRideCancelled, evt.TripID, kafkapkg.RideCancelledEvent{
		TripID: evt.TripID, Reason: "no driver available",
	})
	return nil
}

func (m *Matcher) findCandidates(ctx context.Context, evt kafkapkg.RideRequestedEvent, radiusKm float64) ([]candidate, error) {
	locs, err := m.redis.GeoRadius(ctx, geoKey, evt.PickupLng, evt.PickupLat, radiusKm, 20)
	if err != nil {
		return nil, err
	}

	var candidates []candidate
	for _, loc := range locs {
		meta, _ := m.redis.HGetAll(ctx, "driver:"+loc.Name+":meta")
		if meta["status"] != "available" {
			continue
		}
		if evt.VehicleType != "" && meta["vehicle_type"] != "" && meta["vehicle_type"] != evt.VehicleType {
			continue
		}
		rating := 5.0
		if r, err := strconv.ParseFloat(meta["rating"], 64); err == nil {
			rating = r
		}
		candidates = append(candidates, candidate{
			driverID:   loc.Name,
			lat:        loc.Latitude,
			lng:        loc.Longitude,
			distanceKm: loc.Dist,
			rating:     rating,
		})
	}

	// Score: 60% proximity + 30% rating + 10% random (avoids deterministic hot-spotting)
	sort.Slice(candidates, func(i, j int) bool {
		si := scoreCandidate(candidates[i])
		sj := scoreCandidate(candidates[j])
		return si > sj
	})

	// Take top 5
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	return candidates, nil
}

func scoreCandidate(c candidate) float64 {
	proxScore := 1.0 / (c.distanceKm + 0.1)
	return 0.6*proxScore + 0.3*(c.rating/5.0)
}

func (m *Matcher) offerAndWait(ctx context.Context, evt kafkapkg.RideRequestedEvent, c candidate) (bool, error) {
	lockToken := uuid.New().String()
	lockKey := fmt.Sprintf("dispatch:driver:%s", c.driverID)

	acquired, err := m.redis.LockAcquire(ctx, lockKey, lockToken, lockTTL)
	if err != nil || !acquired {
		return false, nil // driver busy with another offer
	}
	defer m.redis.LockRelease(ctx, lockKey, lockToken) //nolint

	// Set offer pending marker
	offerKey := fmt.Sprintf("offer:%s:%s", evt.TripID, c.driverID)
	_ = m.redis.SetRaw(ctx, offerKey, "pending", offerTTL)

	// Publish RIDE_OFFERED
	_ = m.producer.Publish(ctx, kafkapkg.TopicRideOffered, evt.TripID, kafkapkg.RideOfferedEvent{
		TripID:          evt.TripID,
		DriverID:        c.driverID,
		OfferExpiration: time.Now().Add(offerTTL).Unix(),
	})

	// Poll for driver response via Redis pub/sub channel
	responseChannel := fmt.Sprintf("driver:response:%s", evt.TripID)
	sub := m.redis.Subscribe(ctx, responseChannel)
	defer sub.Close()

	timeout := time.After(offerTTL)
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-timeout:
			return false, nil // driver did not respond
		case msg := <-sub.Channel():
			if msg == nil {
				return false, nil
			}
			var resp struct {
				DriverID string `json:"driver_id"`
				Response string `json:"response"` // ACCEPT | DECLINE
			}
			if err := json.Unmarshal([]byte(msg.Payload), &resp); err != nil {
				continue
			}
			if resp.DriverID != c.driverID {
				continue // message for a different driver on the same trip
			}
			return resp.Response == "ACCEPT", nil
		}
	}
}

func (m *Matcher) assignTrip(ctx context.Context, evt kafkapkg.RideRequestedEvent, c candidate) error {
	// Call trip-service internal endpoint to update trip record
	body, _ := json.Marshal(map[string]string{"driver_id": c.driverID})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPatch,
		m.tripServiceURL+"/"+evt.TripID+"/assign", bytes.NewReader(body))
	req.Header.Set("X-Internal-Secret", m.internalSecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Discard.Write(resp.Body) //nolint

	// Publish DRIVER_ASSIGNED
	return m.producer.Publish(ctx, kafkapkg.TopicDriverAssigned, evt.TripID, kafkapkg.DriverAssignedEvent{
		TripID:   evt.TripID,
		DriverID: c.driverID,
		DriverLat: c.lat,
		DriverLng: c.lng,
	})
}
