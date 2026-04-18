package otp

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	redispkg "github.com/ride-hailing/shared/pkg/redis"
)

const ttl = 10 * time.Minute

func Generate() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func Store(ctx context.Context, rc *redispkg.Client, email, code string) error {
	return rc.SetRaw(ctx, otpKey(email), code, ttl)
}

func Verify(ctx context.Context, rc *redispkg.Client, email, code string) (bool, error) {
	stored, err := rc.GetRaw(ctx, otpKey(email))
	if err != nil {
		return false, nil // key missing = expired
	}
	if stored != code {
		return false, nil
	}
	_ = rc.Del(ctx, otpKey(email))
	return true, nil
}

func otpKey(email string) string { return "otp:" + email }
