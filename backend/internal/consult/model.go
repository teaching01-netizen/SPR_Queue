package consult

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

type Problem struct {
	Code    string
	Message string
	Status  int
}

func (e *Problem) Error() string                     { return e.Message }
func problem(code, message string, status int) error { return &Problem{code, message, status} }

var errNotFound = problem("BOOKING_NOT_FOUND", "ไม่พบข้อมูลการจอง", 404)

func stamp() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func digest(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func seal(key []byte, value string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	n := make([]byte, g.NonceSize())
	if _, err = io.ReadFull(rand.Reader, n); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(g.Seal(n, n, []byte(value), nil)), nil
}
func unseal(key []byte, value string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(b) < g.NonceSize() {
		return "", errors.New("invalid ciphertext")
	}
	out, err := g.Open(nil, b[:g.NonceSize()], b[g.NonceSize():], nil)
	return string(out), err
}

type Input struct {
	Email            string   `json:"email"`
	Wcode            string   `json:"wcode"`
	Nickname         string   `json:"nickname"`
	School           string   `json:"school"`
	Level            string   `json:"level"`
	Subject          string   `json:"subject"`
	Date             string   `json:"date"`
	PeriodID         string   `json:"periodId"`
	Questions        []string `json:"questions"`
	SeminarConfirmed bool     `json:"seminarConfirmed"`
}
type Stage struct {
	CalledAt  string `json:"calledAt,omitempty"`
	StartedAt string `json:"startedAt,omitempty"`
	EndedAt   string `json:"endedAt,omitempty"`
}
type Booking struct {
	Input
	ID                 string           `json:"id"`
	EventID            string           `json:"eventId"`
	RequestID          string           `json:"requestId,omitempty"`
	QueueNo            *string          `json:"queueNo"`
	Orders             map[string]int   `json:"orders"`
	CurrentSubject     string           `json:"currentSubject"`
	Status             string           `json:"status"`
	CheckedInAt        *string          `json:"checkedInAt"`
	Stages             map[string]Stage `json:"stages"`
	ArchivedAt         string           `json:"archivedAt,omitempty"`
	ArchivedBy         string           `json:"archivedBy,omitempty"`
	CompletedAt        string           `json:"completedAt,omitempty"`
	CancelledAt        string           `json:"cancelledAt,omitempty"`
	CancellationReason string           `json:"cancellationReason,omitempty"`
	Mode               string           `json:"mode"`
	Counter            string           `json:"counter"`
	Staff              string           `json:"staff"`
	CreatedAt          string           `json:"createdAt"`
	UpdatedAt          string           `json:"updatedAt"`
	EmailStatus        string           `json:"emailStatus"`
	Revision           int64            `json:"revision"`
	Token              string           `json:"-"`
}
type Confirmation struct {
	Booking     Booking `json:"booking"`
	Token       string  `json:"token"`
	TrackingURL string  `json:"trackingUrl"`
}
type Scope struct {
	Level    string `json:"level"`
	Date     string `json:"date"`
	PeriodID string `json:"periodId"`
	Stream   string `json:"stream"`
}
type LiveQueue struct {
	QueueNo     string `json:"queueNo"`
	State       string `json:"state"`
	Counter     string `json:"counter,omitempty"`
	PeriodStart string `json:"periodStart,omitempty"`
	PeriodEnd   string `json:"periodEnd,omitempty"`
}
type ParentView struct {
	QueuesUntilTurn *int        `json:"queuesUntilTurn"`
	QueueCountState string      `json:"queueCountState"`
	QueueNo         *string     `json:"queueNo"`
	CurrentQueues   []LiveQueue `json:"currentQueues"`
	CurrentSubject  string      `json:"currentSubject"`
	Status          string      `json:"status"`
	Counter         string      `json:"counter"`
}
type StaffView struct {
	CanManageTestData bool      `json:"canManageTestData"`
	Bookings          []Booking `json:"bookings"`
	Revision          int64     `json:"revision"`
}
type Audit struct {
	ID        int64  `json:"id"`
	BookingID string `json:"bookingId"`
	Action    string `json:"action"`
	Actor     string `json:"actor"`
	Previous  string `json:"previous"`
	Next      string `json:"next"`
	At        string `json:"at"`
}

func state(b Booking) string       { return b.CurrentSubject + "/" + b.Status }
func encode(v any) (string, error) { b, err := json.Marshal(v); return string(b), err }
func scopeKey(s Scope) string {
	return fmt.Sprintf("%s/%s/%s/%s", s.Level, s.Date, s.PeriodID, s.Stream)
}
