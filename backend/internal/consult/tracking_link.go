package consult

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
)

type StaffTrackingLink struct {
	TrackingURL string `json:"trackingUrl"`
}

// Recover the existing private link on demand, without changing the booking or token.
// Callers must authenticate Staff before exposing this result.
func (s *Service) StaffTrackingLink(ctx context.Context, id string) (StaffTrackingLink, error) {
	var encrypted, hash string
	err := s.DB.QueryRowContext(ctx, `SELECT token_cipher,token_hash FROM bookings WHERE id=$1 AND event_id=$2`, id, s.Config.Event.ID).Scan(&encrypted, &hash)
	if err == sql.ErrNoRows {
		return StaffTrackingLink{}, errNotFound
	}
	if err != nil {
		return StaffTrackingLink{}, err
	}
	token, err := unseal(s.Key, encrypted)
	if err != nil {
		return StaffTrackingLink{}, err
	}
	if len(token) < 32 || len(token) > 100 || digest(token) != hash {
		return StaffTrackingLink{}, fmt.Errorf("invalid stored tracking token")
	}
	return StaffTrackingLink{TrackingURL: strings.TrimRight(s.PublicURL, "/") + "/#/track/" + url.PathEscape(token)}, nil
}
