package consult

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"strings"
	"time"
)

type EmailContent struct{ Text, HTML string }
type emailDetails struct {
	Nickname, Subject, Date, Start, End, Mode, Venue, Tracking string
	Seminars                                                   []emailSeminar
	Test                                                       bool
}
type emailSeminar struct{ Subject, Date, Start, End string }

func emailSubject(subject string) string {
	switch subject {
	case "Math":
		return "SAT Math"
	case "Verbal":
		return "SAT Verbal"
	default:
		return "SAT Math + SAT Verbal"
	}
}
func emailDate(day string) string {
	date, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	return date.Format("2 January 2006")
}

// RenderConfirmation is shared by actual SMTP delivery and the downloadable preview.
func RenderConfirmation(cfg Config, b Booking, tracking string) (EmailContent, error) {
	_, period, err := cfg.Match(b.Input)
	if err != nil {
		return EmailContent{}, err
	}
	u, err := url.Parse(tracking)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return EmailContent{}, fmt.Errorf("invalid tracking URL")
	}
	d := emailDetails{Nickname: b.Nickname, Subject: emailSubject(b.Subject), Date: emailDate(b.Date), Start: period.Start, End: period.End, Mode: b.Mode, Venue: cfg.Event.Venue, Tracking: tracking, Test: b.ID == "smtp-test"}
	if b.Level == "Beginner" {
		for _, s := range cfg.Seminars {
			if b.Subject == "Combined" || s.Subject == b.Subject {
				d.Seminars = append(d.Seminars, emailSeminar{emailSubject(s.Subject), emailDate(s.Date), s.Start, s.End})
			}
		}
	}
	var html bytes.Buffer
	if err = confirmationTemplate.Execute(&html, d); err != nil {
		return EmailContent{}, err
	}
	var text strings.Builder
	if d.Test {
		text.WriteString("อีเมลทดสอบระบบ — ไม่มีการจองหรือรหัสคิวจริง\n\n")
	}
	fmt.Fprintf(&text, "เรียน ผู้ปกครองของน้อง %s\n\nยืนยันการลงทะเบียน Private Consult กับ Warwick Institute\n", d.Nickname)
	for _, s := range d.Seminars {
		fmt.Fprintf(&text, "\nสัมมนารวม: %s\nวันที่: %s\nเวลาสัมมนา: %s - %s น.\n", s.Subject, s.Date, s.Start, s.End)
	}
	fmt.Fprintf(&text, "\nPrivate Consult - การปรึกษารายบุคคล\nวันที่: %s\nวิชา: %s\nช่วงเวลา Consult: %s - %s\nรูปแบบ: %s\n", d.Date, d.Subject, d.Start, d.End, d.Mode)
	if d.Mode == "On-site" {
		fmt.Fprintf(&text, "สถานที่: %s\n", d.Venue)
	}
	text.WriteString("\nช่วงเวลาที่จองเป็นรอบ Consult ทีมงานจะจัดลำดับคิวภายในรอบ กรุณาติดตามสถานะก่อนเข้ารับการปรึกษา\nหากมีการเปลี่ยนแปลงกำหนดการ ทางสถาบันจะแจ้งให้ทราบ\n\nตรวจสอบสถานะคิวของท่าน\nCheck Queue Status: " + tracking + "\nกรุณาเก็บลิงก์นี้ไว้สำหรับตรวจสอบคิวของท่าน\n\nขอแสดงความนับถือ\nWarwick Institute\n")
	return EmailContent{Text: text.String(), HTML: html.String()}, nil
}

var confirmationTemplate = template.Must(template.New("confirmation").Parse(`<!doctype html>
<html lang="th"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Warwick Private Consult</title></head>
<body style="margin:0;padding:0;background:#ffffff;color:#252525;font-family:Arial,Tahoma,sans-serif;line-height:1.8;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0"><tr><td style="padding:24px 16px;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="max-width:760px;margin:0 auto;font-size:16px;">
{{if .Test}}<tr><td style="padding:12px;background:#fff3c4;"><strong>อีเมลทดสอบระบบ — ไม่มีการจองหรือรหัสคิวจริง</strong></td></tr>{{end}}
<tr><td style="padding:8px 0 20px;"><strong>เรียน ผู้ปกครองของน้อง {{.Nickname}}</strong></td></tr>
<tr><td style="padding:0 0 24px;">ตามที่ท่านได้ลงทะเบียนเข้าร่วมกิจกรรม <strong>Student Performance Report</strong> กับ Warwick Institute ทางสถาบันขอยืนยันรายละเอียดการเข้าร่วมกิจกรรมดังนี้</td></tr>
{{if .Seminars}}<tr><td style="padding-bottom:20px;"><table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#f7f7f7;border-left:5px solid #920000;border-radius:12px;"><tr><td style="padding:20px 24px;"><h2 style="color:#920000;font-size:20px;margin:0 0 16px;">ช่วงที่ 1: สัมมนารวม</h2>
{{range .Seminars}}<p style="margin:0 0 12px;"><strong>วันที่:</strong> {{.Date}}<br><strong>วิชา:</strong> {{.Subject}}<br><strong>เวลาสัมมนา:</strong> {{.Start}} - {{.End}} น.</p>{{end}}
</td></tr></table></td></tr>{{end}}
<tr><td style="padding-bottom:24px;"><table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#f7f7f7;border-left:5px solid #920000;border-radius:12px;"><tr><td style="padding:20px 24px;"><h2 style="color:#920000;font-size:20px;margin:0 0 16px;">{{if .Seminars}}ช่วงที่ 2: {{end}}Private Consult - การปรึกษารายบุคคล</h2>
<p style="margin:0 0 8px;"><strong>วันที่:</strong> {{.Date}}<br><strong>วิชา:</strong> {{.Subject}}</p>
<p style="margin:12px 0;"><strong>ช่วงเวลา Consult:</strong> <span style="display:inline-block;padding:4px 10px;background:#fff2a6;border-radius:7px;font-weight:bold;">{{.Start}} - {{.End}}</span></p>
<p style="margin:0;"><strong>รูปแบบ:</strong> {{.Mode}}{{if eq .Mode "On-site"}}<br><strong>สถานที่:</strong> {{.Venue}}{{end}}</p>
</td></tr></table></td></tr>
<tr><td style="padding:0 0 16px;">ช่วงเวลาที่จองเป็นรอบ Consult ทีมงานจะจัดลำดับคิวภายในรอบ กรุณาติดตามสถานะก่อนเข้ารับการปรึกษา</td></tr>
<tr><td style="padding:0 0 24px;">หากมีการเปลี่ยนแปลงกำหนดการ ทางสถาบันจะแจ้งให้ท่านทราบ</td></tr>
<tr><td style="padding-bottom:28px;"><table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#f7f7f7;border:1px solid #dedede;border-radius:12px;"><tr><td style="padding:20px 24px;">
<strong>ตรวจสอบสถานะคิวของท่าน</strong><p style="margin:8px 0 18px;">สามารถกดปุ่มด้านล่างเพื่อตรวจสอบสถานะคิวในวันงาน</p>
<table role="presentation" cellspacing="0" cellpadding="0"><tr><td bgcolor="#920000" style="border-radius:28px;"><a href="{{.Tracking}}" style="display:inline-block;background:#920000;border:1px solid #920000;border-radius:28px;color:#ffffff;text-decoration:none;padding:12px 22px;font-weight:bold;">Check Queue Status</a></td></tr></table>
<p style="margin:14px 0 0;font-size:14px;color:#666666;">หากกดปุ่มไม่ได้ สามารถคัดลอกลิงก์นี้ไปเปิดใน Browser ได้:<br><a href="{{.Tracking}}" style="color:#135dc6;word-break:break-all;overflow-wrap:anywhere;">{{.Tracking}}</a></p>
<p style="margin:10px 0 0;font-size:13px;color:#666666;">กรุณาเก็บลิงก์ส่วนตัวนี้ไว้สำหรับตรวจสอบคิวของท่าน</p>
</td></tr></table></td></tr>
<tr><td>ขอแสดงความนับถือ<br><br><strong>Warwick Institute</strong></td></tr>
</table></td></tr></table></body></html>`))
