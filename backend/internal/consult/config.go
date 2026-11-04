package consult

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type Event struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Cycle    string `json:"cycle"`
	Venue    string `json:"venue"`
	Sample   bool   `json:"sample"`
	Timezone string `json:"timezone"`
}
type Period struct {
	ID       string `json:"id"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Capacity int    `json:"capacity"`
}
type Rule struct {
	Level   string   `json:"level"`
	Subject string   `json:"subject"`
	Date    string   `json:"date"`
	Mode    string   `json:"mode"`
	Periods []Period `json:"periods"`
}
type Seminar struct {
	Subject string `json:"subject"`
	Date    string `json:"date"`
	Start   string `json:"start"`
	End     string `json:"end"`
}
type Config struct {
	Event    Event     `json:"event"`
	Rules    []Rule    `json:"rules"`
	Seminars []Seminar `json:"seminars,omitempty"`
}

var identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

func LoadConfig(path string) (Config, error) {
	var c Config
	f, err := os.Open(path)
	if err != nil {
		return c, err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil {
		return c, err
	}
	return c, c.Validate()
}
func (c Config) Validate() error {
	if !identifier.MatchString(c.Event.ID) || c.Event.Title == "" || len(c.Rules) == 0 {
		return fmt.Errorf("invalid event configuration")
	}
	if _, err := time.LoadLocation(c.Event.Timezone); err != nil {
		return err
	}
	seminarSubjects := map[string]bool{}
	for _, seminar := range c.Seminars {
		start, ae := time.Parse("15:04", seminar.Start)
		end, be := time.Parse("15:04", seminar.End)
		_, de := time.Parse("2006-01-02", seminar.Date)
		if (seminar.Subject != "Math" && seminar.Subject != "Verbal") || seminarSubjects[seminar.Subject] || ae != nil || be != nil || de != nil || !start.Before(end) {
			return fmt.Errorf("invalid seminar configuration")
		}
		seminarSubjects[seminar.Subject] = true
	}
	seen := map[string]bool{}
	labels := map[string]string{}
	for _, r := range c.Rules {
		if (r.Level != "Beginner" && r.Level != "High Level") || !validSubject(r.Subject) || (r.Mode != "On-site" && r.Mode != "Online via Zoom") {
			return fmt.Errorf("invalid level, subject or mode")
		}
		if _, err := time.Parse("2006-01-02", r.Date); err != nil {
			return err
		}
		if len(r.Periods) == 0 {
			return fmt.Errorf("rule has no periods")
		}
		for _, p := range r.Periods {
			a, ae := time.Parse("15:04", p.Start)
			b, be := time.Parse("15:04", p.End)
			k := r.Level + "/" + r.Subject + "/" + r.Date + "/" + p.ID
			if !identifier.MatchString(p.ID) || p.Capacity < 1 || p.Capacity > 100 || ae != nil || be != nil || !a.Before(b) || seen[k] {
				return fmt.Errorf("invalid or duplicate period %s", k)
			}
			labelKey := r.Level + "/" + r.Date + "/" + p.ID
			label := p.Start + "/" + p.End
			if previous, ok := labels[labelKey]; ok && previous != label {
				return fmt.Errorf("period ID has inconsistent times across subjects")
			}
			labels[labelKey] = label
			seen[k] = true
		}
	}
	return nil
}
func validSubject(s string) bool { return s == "Math" || s == "Verbal" || s == "Combined" }
func (c Config) Match(i Input) (Rule, Period, error) {
	for _, r := range c.Rules {
		if r.Level == i.Level && r.Subject == i.Subject && r.Date == i.Date {
			for _, p := range r.Periods {
				if p.ID == i.PeriodID {
					return r, p, nil
				}
			}
		}
	}
	return Rule{}, Period{}, problem("INVALID_PERIOD", "ระดับ วิชา วัน หรือรอบที่เลือกไม่ได้เปิดให้จอง", 400)
}
func (c Config) Normalize(i Input) (Input, Rule, Period, error) {
	i.Email = strings.ToLower(strings.TrimSpace(i.Email))
	i.Nickname = strings.TrimSpace(i.Nickname)
	i.School = strings.TrimSpace(i.School)
	i.Wcode = strings.TrimSpace(i.Wcode)
	r, p, err := c.Match(i)
	if err != nil {
		return i, r, p, err
	}
	i, err = normalizeStudentDetails(i)
	if err != nil || len(i.Questions) < 2 || len(i.Questions) > 3 {
		return i, r, p, problem("VALIDATION_ERROR", "กรุณากรอกข้อมูลผู้ปกครองและนักเรียนให้ถูกต้อง", 400)
	}
	// Normalize an owned slice; callers may reuse an Input across concurrent requests.
	i.Questions = append([]string(nil), i.Questions...)
	for n, q := range i.Questions {
		i.Questions[n] = strings.TrimSpace(q)
		if utf8.RuneCountInString(q) > 1000 {
			return i, r, p, problem("VALIDATION_ERROR", "คำถามยาวเกินกำหนด", 400)
		}
	}
	if i.Questions[0] == "" || i.Questions[1] == "" || (i.Level == "Beginner" && !i.SeminarConfirmed) {
		return i, r, p, problem("VALIDATION_ERROR", "กรุณากรอกคำถาม 2 ข้อและยืนยันสัมมนา Beginner", 400)
	}
	if !c.Event.Sample {
		loc, _ := time.LoadLocation(c.Event.Timezone)
		if i.Date < time.Now().In(loc).Format("2006-01-02") {
			return i, r, p, problem("EVENT_CLOSED", "วันกิจกรรมนี้ผ่านไปแล้ว", 400)
		}
	}
	return i, r, p, nil
}
