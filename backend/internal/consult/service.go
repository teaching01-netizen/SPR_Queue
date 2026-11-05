package consult

import (
	"context"
	"database/sql"
	"net/url"
	"sort"
	"strings"
)

func (s *Service) Book(ctx context.Context, key string, raw Input) (Confirmation, error) {
	if !identifier.MatchString(key) {
		return Confirmation{}, problem("VALIDATION_ERROR", "ไม่พบรหัสคำขอที่ถูกต้อง", 400)
	}
	i, rule, p, err := s.Config.Normalize(raw)
	if err != nil {
		return Confirmation{}, err
	}
	text, err := encode(i)
	if err != nil {
		return Confirmation{}, err
	}
	payloadHash := digest(text)
	tx, err := s.beginWrite(ctx)
	if err != nil {
		return Confirmation{}, err
	}
	defer tx.Rollback()
	var previousID, previousHash, ciphertext string
	err = tx.QueryRowContext(ctx, `SELECT id,payload_hash,token_cipher FROM bookings WHERE event_id=$1 AND request_key=$2`, s.Config.Event.ID, key).Scan(&previousID, &previousHash, &ciphertext)
	if err == nil {
		if previousHash != payloadHash {
			return Confirmation{}, problem("IDEMPOTENCY_CONFLICT", "รหัสคำขอนี้ถูกใช้กับข้อมูลอื่นแล้ว กรุณาเริ่มคำขอใหม่", 409)
		}
		b, e := getBooking(ctx, tx, previousID)
		if e != nil {
			return Confirmation{}, e
		}
		token, e := unseal(s.Key, ciphertext)
		if e != nil {
			return Confirmation{}, e
		}
		return s.confirm(b, token), nil
	}
	if err != sql.ErrNoRows {
		return Confirmation{}, err
	}
	var count int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings WHERE event_id=$1 AND level=$2 AND subject=$3 AND date=$4 AND period_id=$5 AND status!='Cancelled'`, s.Config.Event.ID, i.Level, i.Subject, i.Date, i.PeriodID).Scan(&count)
	if err != nil {
		return Confirmation{}, err
	}
	if count >= p.Capacity {
		return Confirmation{}, problem("PERIOD_FULL", "ช่วงเวลานี้เต็มแล้ว กรุณาเลือกช่วงอื่น", 409)
	}
	id, err := randomToken()
	if err != nil {
		return Confirmation{}, err
	}
	token, err := randomToken()
	if err != nil {
		return Confirmation{}, err
	}
	ciphertext, err = seal(s.Key, token)
	if err != nil {
		return Confirmation{}, err
	}
	current := "Math"
	if i.Subject == "Verbal" {
		current = "Verbal"
	}
	emailStatus := "Not configured"
	if s.MailEnabled {
		emailStatus = "Pending"
	}
	b := Booking{Input: i, ID: id, EventID: s.Config.Event.ID, RequestID: key, Orders: map[string]int{}, CurrentSubject: current, Status: "Booked", Stages: map[string]Stage{}, Mode: rule.Mode, CreatedAt: stamp(), UpdatedAt: stamp(), Revision: 1, EmailStatus: emailStatus}
	data, err := encode(b)
	if err != nil {
		return Confirmation{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO bookings(id,event_id,level,subject,date,period_id,current_subject,status,queue_no,request_key,payload_hash,token_hash,token_cipher,data) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, b.ID, b.EventID, b.Level, b.Subject, b.Date, b.PeriodID, b.CurrentSubject, b.Status, nil, key, payloadHash, digest(token), ciphertext, data)
	if err != nil {
		return Confirmation{}, err
	}
	if s.MailEnabled {
		if _, err = tx.ExecContext(ctx, `INSERT INTO outbox(booking_id,state) VALUES($1,'Pending')`, b.ID); err != nil {
			return Confirmation{}, err
		}
	}
	if err = addAudit(ctx, tx, b, "BOOK", "", "Participant"); err != nil {
		return Confirmation{}, err
	}
	if err = bump(ctx, tx); err != nil {
		return Confirmation{}, err
	}
	if err = tx.Commit(); err != nil {
		return Confirmation{}, err
	}
	return s.confirm(b, token), nil
}
func (s *Service) confirm(b Booking, token string) Confirmation {
	return Confirmation{b, token, strings.TrimRight(s.PublicURL, "/") + "/#/track/" + url.PathEscape(token)}
}

type Availability struct {
	ID        string `json:"id"`
	Start     string `json:"start"`
	End       string `json:"end"`
	Available bool   `json:"available"`
}

func (s *Service) Periods(ctx context.Context, level, subject, date string) ([]Availability, error) {
	out := []Availability{}
	var rule *Rule
	for n := range s.Config.Rules {
		r := &s.Config.Rules[n]
		if r.Level == level && r.Subject == subject && r.Date == date {
			rule = r
			break
		}
	}
	if rule == nil {
		return out, problem("INVALID_PERIOD", "ไม่มีรอบสำหรับเงื่อนไขนี้", 400)
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT period_id,COUNT(*) FROM bookings WHERE event_id=$1 AND level=$2 AND subject=$3 AND date=$4 AND status!='Cancelled' GROUP BY period_id`, s.Config.Event.ID, level, subject, date)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err = rows.Scan(&id, &n); err != nil {
			return out, err
		}
		counts[id] = n
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	for _, p := range rule.Periods {
		out = append(out, Availability{p.ID, p.Start, p.End, counts[p.ID] < p.Capacity})
	}
	return out, nil
}
func (s *Service) Assign(ctx context.Context, scope Scope, ids []string, revision int64, actor string) error {
	if scope.Stream != "Math" && scope.Stream != "Verbal" {
		return problem("VALIDATION_ERROR", "วิชาปัจจุบันไม่ถูกต้อง", 400)
	}
	tx, err := s.beginWrite(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current int64
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM meta WHERE id=1`).Scan(&current); err != nil {
		return err
	}
	if revision != current {
		return problem("REVISION_CONFLICT", "ข้อมูลคิวเปลี่ยนแล้ว กรุณารีเฟรชและจัดลำดับใหม่", 409)
	}
	rows, err := tx.QueryContext(ctx, `SELECT data FROM bookings WHERE event_id=$1 AND level=$2 AND date=$3 AND period_id=$4 AND current_subject=$5 AND status IN ('Booked','Waiting')`, s.Config.Event.ID, scope.Level, scope.Date, scope.PeriodID, scope.Stream)
	if err != nil {
		return err
	}
	eligible := map[string]Booking{}
	for rows.Next() {
		b, e := scanBooking(rows)
		if e != nil {
			rows.Close()
			return e
		}
		eligible[b.ID] = b
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(eligible) != len(ids) || len(ids) == 0 {
		return problem("STALE_QUEUE", "กรุณาบันทึกรายการคิวที่รอให้ครบทั้งรอบ", 409)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if _, ok := eligible[id]; !ok || seen[id] {
			return problem("STALE_QUEUE", "รายการคิวไม่ถูกต้องหรือซ้ำ", 409)
		}
		seen[id] = true
	}
	// Older cancelled rows can still hold a code. Release them atomically before
	// finding a free number; active and completed bookings are never renumbered.
	if err = releaseCancelledQueueCodes(ctx, tx, s.Config.Event.ID, actor); err != nil {
		return err
	}

	for n, id := range ids {
		b := eligible[id]
		previous := state(b)
		if b.QueueNo == nil {
			code, e := nextAvailableQueueCode(ctx, tx, b)
			if e != nil {
				return e
			}
			b.QueueNo = &code
		}
		b.Orders[scope.Stream] = n + 1
		b.Staff = actor
		if err = updateBooking(ctx, tx, &b); err != nil {
			return err
		}
		if err = addAudit(ctx, tx, b, "ASSIGN_QUEUE", previous, actor); err != nil {
			return err
		}
	}
	if err = bump(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

var allowed = map[string][]string{"Booked": {"Waiting", "Cancelled"}, "Waiting": {"Calling", "Cancelled", "No-show"}, "Calling": {"Serving", "Waiting", "No-show", "Cancelled"}, "Serving": {"Completed"}, "Completed": {}, "No-show": {"Waiting", "Cancelled"}, "Cancelled": {}}

func contains(a []string, b string) bool {
	for _, v := range a {
		if v == b {
			return true
		}
	}
	return false
}
func (s *Service) Status(ctx context.Context, id, next string, revision int64, actor, reason string) (Booking, error) {
	tx, err := s.beginWrite(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback()
	b, err := getBooking(ctx, tx, id)
	if err != nil {
		return b, err
	}
	if b.EventID != s.Config.Event.ID {
		return Booking{}, errNotFound
	}
	if next == "Waiting" && b.Status == "Waiting" {
		return b, nil
	}
	if revision != b.Revision {
		return b, problem("REVISION_CONFLICT", "รายการนี้เปลี่ยนแล้ว กรุณารีเฟรช", 409)
	}
	if !contains(allowed[b.Status], next) {
		return b, problem("INVALID_STATUS_TRANSITION", "เปลี่ยนสถานะนี้ไม่ได้", 409)
	}
	if next == "Calling" {
		if b.QueueNo == nil || b.Orders[b.CurrentSubject] == 0 || b.CheckedInAt == nil {
			return b, problem("QUEUE_NOT_READY", "กรุณาจัดคิวและเช็กอินก่อนเรียก", 409)
		}
		var count int
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings WHERE event_id=$1 AND level=$2 AND date=$3 AND current_subject=$4 AND status IN ('Calling','Serving') AND id!=$5`, b.EventID, b.Level, b.Date, b.CurrentSubject, b.ID).Scan(&count)
		if err != nil {
			return b, err
		}
		if count > 0 {
			return b, problem("STREAM_BUSY", "มีคิวกำลังเรียกหรือปรึกษาในวิชานี้ กรุณาจบคิวเดิมก่อน", 409)
		}
	}
	reason = strings.TrimSpace(reason)
	if next == "Cancelled" && (reason == "" || len(reason) > 2000) {
		return b, problem("VALIDATION_ERROR", "กรุณาระบุเหตุผลที่ยกเลิก", 400)
	}
	previous := state(b)
	if next == "Cancelled" {
		previous = bookingManagementState(b)
	}
	timestamp := stamp()
	stage := b.Stages[b.CurrentSubject]
	action := "STATUS_" + next
	if b.Status == "No-show" && next == "Waiting" {
		action = "RECOVER_NO_SHOW"
	}
	if next == "Waiting" && b.CheckedInAt == nil {
		b.CheckedInAt = &timestamp
	}
	if next == "Calling" {
		stage.CalledAt = timestamp
		b.Counter = b.CurrentSubject + " Room"
	}
	if next == "Serving" {
		stage.StartedAt = timestamp
	}
	if next == "Completed" {
		stage.EndedAt = timestamp
	}
	b.Stages[b.CurrentSubject] = stage
	other := "Math"
	if b.CurrentSubject == "Math" {
		other = "Verbal"
	}
	if next == "Completed" && b.Subject == "Combined" && b.Stages[other].EndedAt == "" {
		b.CurrentSubject = other
		b.Status = "Waiting"
		b.Counter = ""
		delete(b.Orders, other)
	} else {
		b.Status = next
		if next == "Completed" {
			b.CompletedAt = timestamp
		}
		if next == "Cancelled" {
			b.CancelledAt = timestamp
			b.CancellationReason = reason
			b.QueueNo = nil
			b.Orders = map[string]int{}
			b.Counter = ""
		}
		if next == "Waiting" {
			b.Counter = ""
		}
	}
	b.Staff = actor
	if err = updateBooking(ctx, tx, &b); err != nil {
		return b, err
	}
	if next == "Cancelled" {
		err = auditBookingChange(ctx, tx, b, action, previous, actor)
	} else {
		err = addAudit(ctx, tx, b, action, previous, actor)
	}
	if err != nil {
		return b, err
	}
	if err = bump(ctx, tx); err != nil {
		return b, err
	}
	return b, tx.Commit()
}

// ChooseStartSubject changes the first room only before either subject has been
// called. Existing queue codes and check-in remain stable; room order is rebuilt.
func (s *Service) ChooseStartSubject(ctx context.Context, id, stream string, revision int64, actor string) (Booking, error) {
	if stream != "Math" && stream != "Verbal" {
		return Booking{}, problem("VALIDATION_ERROR", "เลือก Math หรือ Verbal เท่านั้น", 400)
	}
	tx, err := s.beginWrite(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback()
	b, err := getBooking(ctx, tx, id)
	if err != nil {
		return b, err
	}
	if b.EventID != s.Config.Event.ID {
		return Booking{}, errNotFound
	}
	if revision != b.Revision {
		return b, problem("REVISION_CONFLICT", "รายการนี้เปลี่ยนแล้ว กรุณารีเฟรช", 409)
	}
	if b.Subject != "Combined" || !contains([]string{"Booked", "Waiting", "No-show"}, b.Status) {
		return b, problem("START_SUBJECT_LOCKED", "เลือกวิชาแรกได้เฉพาะ Math + Verbal ก่อนเรียกคิวครั้งแรก", 409)
	}
	for _, stage := range b.Stages {
		if stage.CalledAt != "" || stage.StartedAt != "" || stage.EndedAt != "" {
			return b, problem("START_SUBJECT_LOCKED", "เรียกคิวแล้ว ไม่สามารถเปลี่ยนวิชาแรกได้", 409)
		}
	}
	if b.CurrentSubject == stream {
		return b, nil
	}
	previous := state(b)
	b.CurrentSubject = stream
	b.Orders = map[string]int{}
	b.Counter = ""
	b.Staff = actor
	if err = updateBooking(ctx, tx, &b); err != nil {
		return b, err
	}
	if err = addAudit(ctx, tx, b, "CHOOSE_START_SUBJECT", previous, actor); err != nil {
		return b, err
	}
	if err = bump(ctx, tx); err != nil {
		return b, err
	}
	return b, tx.Commit()
}
func (s *Service) Track(ctx context.Context, token string) (ParentView, error) {
	var id string
	if len(token) < 32 || len(token) > 100 {
		return ParentView{}, errNotFound
	}
	err := s.DB.QueryRowContext(ctx, `SELECT id FROM bookings WHERE token_hash=$1 AND event_id=$2`, digest(token), s.Config.Event.ID).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return ParentView{}, errNotFound
		}
		return ParentView{}, err
	}
	view, err := s.snapshot(ctx)
	if err != nil {
		return ParentView{}, err
	}
	var booking *Booking
	for n := range view.Bookings {
		if view.Bookings[n].ID == id {
			booking = &view.Bookings[n]
			break
		}
	}
	if booking == nil {
		return ParentView{}, errNotFound
	}
	b := booking
	remaining, countState := s.queuesUntilTurn(view.Bookings, *b)
	out := ParentView{QueuesUntilTurn: remaining, QueueCountState: countState, QueueNo: b.QueueNo, CurrentQueues: []LiveQueue{}, CurrentSubject: b.CurrentSubject, Status: b.Status, Counter: b.Counter}
	for _, other := range view.Bookings {
		if other.Level == b.Level && other.Date == b.Date && other.CurrentSubject == b.CurrentSubject && other.QueueNo != nil && contains([]string{"Calling", "Serving"}, other.Status) {
			out.CurrentQueues = append(out.CurrentQueues, LiveQueue{QueueNo: *other.QueueNo, State: other.Status})
		}
	}
	return out, nil
}
func (s *Service) Display(ctx context.Context, level, date string) (map[string][]LiveQueue, error) {
	view, err := s.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string][]LiveQueue{"Math": {}, "Verbal": {}, "MathNext": s.upcoming(view.Bookings, level, date, "Math"), "VerbalNext": s.upcoming(view.Bookings, level, date, "Verbal")}
	for _, b := range view.Bookings {
		if b.Level == level && b.Date == date && b.QueueNo != nil && contains([]string{"Calling", "Serving"}, b.Status) {
			out[b.CurrentSubject] = append(out[b.CurrentSubject], LiveQueue{QueueNo: *b.QueueNo, State: b.Status, Counter: b.Counter})
		}
	}
	return out, nil
}
func (s *Service) Audit(ctx context.Context) ([]Audit, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT a.id,a.booking_id,a.action,a.actor,a.previous,a.next,a.at FROM audit a JOIN bookings b ON a.booking_id=b.id WHERE b.event_id=$1 ORDER BY a.id DESC LIMIT 200`, s.Config.Event.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Audit{}
	for rows.Next() {
		var a Audit
		if err = rows.Scan(&a.ID, &a.BookingID, &a.Action, &a.Actor, &a.Previous, &a.Next, &a.At); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Service) Reschedule(ctx context.Context, id, date, periodID string, revision int64, actor string) (Booking, error) {
	tx, err := s.beginWrite(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback()
	b, err := getBooking(ctx, tx, id)
	if err != nil {
		return b, err
	}
	if b.EventID != s.Config.Event.ID {
		return Booking{}, errNotFound
	}
	if b.Revision != revision {
		return b, problem("REVISION_CONFLICT", "รายการนี้เปลี่ยนแล้ว กรุณารีเฟรช", 409)
	}
	if b.Status != "Booked" {
		return b, problem("INVALID_STATUS_TRANSITION", "เลื่อนรอบได้เฉพาะรายการที่ยังไม่เช็กอิน", 409)
	}
	if date == b.Date && periodID == b.PeriodID {
		return b, nil
	}
	var deliveryState string
	if e := tx.QueryRowContext(ctx, `SELECT state FROM outbox WHERE booking_id=$1`, id).Scan(&deliveryState); e != nil && e != sql.ErrNoRows {
		return b, e
	}
	if deliveryState == "Sending" {
		return b, problem("NOTIFICATION_BUSY", "ระบบกำลังส่งอีเมล กรุณาลองเลื่อนรอบอีกครั้งภายหลัง", 409)
	}
	i := b.Input
	i.Date = date
	i.PeriodID = periodID
	_, r, p, err := s.Config.Normalize(i)
	if err != nil {
		return b, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings WHERE event_id=$1 AND level=$2 AND subject=$3 AND date=$4 AND period_id=$5 AND status!='Cancelled' AND id!=$6`, b.EventID, b.Level, b.Subject, date, periodID, id).Scan(&count); err != nil {
		return b, err
	}
	if count >= p.Capacity {
		return b, problem("PERIOD_FULL", "รอบใหม่เต็มแล้ว", 409)
	}
	previous := state(b)
	b.Date = date
	b.PeriodID = periodID
	b.Mode = r.Mode
	b.Orders = map[string]int{}
	b.Staff = actor
	if err = updateBooking(ctx, tx, &b); err != nil {
		return b, err
	}
	if err = addAudit(ctx, tx, b, "RESCHEDULE", previous, actor); err != nil {
		return b, err
	}
	if s.MailEnabled {
		if _, err = tx.ExecContext(ctx, `INSERT INTO outbox(booking_id,state) VALUES($1,'Pending') ON CONFLICT(booking_id) DO UPDATE SET state='Pending',attempts=0,next_at=0`, id); err != nil {
			return b, err
		}
		b.EmailStatus = "Pending"
		if err = updateBooking(ctx, tx, &b); err != nil {
			return b, err
		}
	}
	if err = bump(ctx, tx); err != nil {
		return b, err
	}
	return b, tx.Commit()
}
func SortBookings(b []Booking, stream string) {
	sort.Slice(b, func(i, j int) bool { return b[i].Orders[stream] < b[j].Orders[stream] })
}
