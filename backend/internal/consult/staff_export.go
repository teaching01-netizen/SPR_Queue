package consult

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const excelContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// StaffExport reads the same event-scoped, consistent snapshot as the dashboard.
// It deliberately ignores UI filters and never changes bookings or queue order.
func (s *Service) StaffExport(ctx context.Context) ([]byte, error) {
	view, err := s.StaffList(ctx)
	if err != nil {
		return nil, err
	}
	return registrationWorkbook(s.Config, view.Bookings)
}

func registrationWorkbook(cfg Config, bookings []Booking) ([]byte, error) {
	sheets, err := registrationSheets(cfg, bookings)
	if err != nil {
		return nil, err
	}
	return xlsxWorkbook(sheets)
}

// Both Excel and Google Sheets use this one column schema and visibility rule.
func registrationSheets(cfg Config, bookings []Booking) ([]exportSheet, error) {
	loc, err := time.LoadLocation(cfg.Event.Timezone)
	if err != nil {
		return nil, err
	}
	localTime := func(value string) string {
		if value == "" {
			return ""
		}
		if t, e := time.Parse(time.RFC3339Nano, value); e == nil {
			return t.In(loc).Format("2006-01-02 15:04:05")
		}
		return value
	}
	subject := func(value string) string {
		if value == "Combined" {
			return "Math + Verbal"
		}
		return value
	}
	statuses := map[string]string{"Booked": "ลงทะเบียนแล้ว", "Waiting": "รอเรียกคิว", "Calling": "กำลังเรียก", "Serving": "กำลังปรึกษา", "Completed": "เสร็จสิ้น", "Cancelled": "ยกเลิก", "No-show": "ไม่มา"}
	label := func(value string) string {
		if text, ok := statuses[value]; ok {
			return text
		}
		return value
	}
	boolean := func(value bool) string {
		if value {
			return "ใช่"
		}
		return "ไม่ใช่"
	}
	queue := func(value *string) string {
		if value != nil {
			return *value
		}
		return ""
	}
	order := func(n int) string {
		if n > 0 {
			return strconv.Itoa(n)
		}
		return ""
	}
	questions := func(b Booking, n int) string {
		if len(b.Questions) > n {
			return b.Questions[n]
		}
		return ""
	}

	type group struct {
		rule           Rule
		period         Period
		total, checked int
		states         map[string]int
	}
	groups := map[string]*group{}
	key := func(i Input) string { return strings.Join([]string{i.Date, i.Level, i.Subject, i.PeriodID}, "|") }
	for _, r := range cfg.Rules {
		for _, p := range r.Periods {
			groups[key(Input{Date: r.Date, Level: r.Level, Subject: r.Subject, PeriodID: p.ID})] = &group{rule: r, period: p, states: map[string]int{}}
		}
	}

	// Work on an owned slice; display order and the caller's snapshot stay intact.
	ordered := make([]Booking, 0, len(bookings))
	for _, b := range bookings {
		if b.Status != "Cancelled" && b.ArchivedAt == "" {
			ordered = append(ordered, b)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if key(a.Input) != key(b.Input) {
			return key(a.Input) < key(b.Input)
		}
		if a.CreatedAt != b.CreatedAt {
			return a.CreatedAt < b.CreatedAt
		}
		return a.ID < b.ID
	})
	rows := [][]string{{"รหัสการจอง", "รหัสคิว", "Wcode", "ชื่อเล่นนักเรียน", "โรงเรียน", "Email ผู้ปกครอง", "ระดับ", "วิชาที่จอง", "วันที่ Consult", "ช่วงเวลา Consult", "รูปแบบ", "สถานะ", "วิชาปัจจุบัน", "ลำดับ Math", "ลำดับ Verbal", "เช็กอินแล้ว", "เวลาเช็กอิน (" + cfg.Event.Timezone + ")", "ยืนยันสัมมนา Beginner", "สัมมนารวม", "คำถาม 1", "คำถาม 2", "คำถาม 3", "ห้อง / เคาน์เตอร์", "Staff", "เวลาลงทะเบียน", "แก้ไขล่าสุด", "เวลาจบเคส", "เวลายกเลิก", "เหตุผลยกเลิก", "เก็บออกจากรายการแล้ว", "เวลาเก็บรายการ", "ผู้เก็บรายการ", "สถานะอีเมล", "Math เรียก", "Math เริ่ม", "Math จบ", "Verbal เรียก", "Verbal เริ่ม", "Verbal จบ"}}
	for _, b := range ordered {
		g := groups[key(b.Input)]
		if g == nil {
			// Keep historical periods even if they are no longer offered for booking.
			g = &group{rule: Rule{Date: b.Date, Level: b.Level, Subject: b.Subject, Mode: b.Mode}, period: Period{ID: b.PeriodID}, states: map[string]int{}}
			groups[key(b.Input)] = g
		}
		g.total++
		g.states[b.Status]++
		if b.CheckedInAt != nil && *b.CheckedInAt != "" {
			g.checked++
		}
		period := g.period.Start + " - " + g.period.End
		if g.period.Start == "" {
			period = "รอบเดิม " + b.PeriodID
		}
		seminars := []string{}
		if b.Level == "Beginner" {
			for _, sem := range cfg.Seminars {
				if b.Subject == "Combined" || b.Subject == sem.Subject {
					seminars = append(seminars, sem.Date+" · "+sem.Subject+" "+sem.Start+" - "+sem.End)
				}
			}
		}
		m, v := b.Stages["Math"], b.Stages["Verbal"]
		rows = append(rows, []string{b.ID, queue(b.QueueNo), b.Wcode, b.Nickname, b.School, b.Email, b.Level, subject(b.Subject), b.Date, period, b.Mode, label(b.Status), subject(b.CurrentSubject), order(b.Orders["Math"]), order(b.Orders["Verbal"]), boolean(b.CheckedInAt != nil && *b.CheckedInAt != ""), localTime(queue(b.CheckedInAt)), boolean(b.SeminarConfirmed), strings.Join(seminars, "\n"), questions(b, 0), questions(b, 1), questions(b, 2), b.Counter, b.Staff, localTime(b.CreatedAt), localTime(b.UpdatedAt), localTime(b.CompletedAt), localTime(b.CancelledAt), b.CancellationReason, boolean(b.ArchivedAt != ""), localTime(b.ArchivedAt), b.ArchivedBy, b.EmailStatus, localTime(m.CalledAt), localTime(m.StartedAt), localTime(m.EndedAt), localTime(v.CalledAt), localTime(v.StartedAt), localTime(v.EndedAt)})
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	summary := [][]string{{"วันที่ Consult", "ระดับ", "วิชาที่จอง", "ช่วงเวลา Consult", "รูปแบบ", "จำนวน Slot", "ผู้ลงทะเบียนที่ส่งออก", "เช็กอินแล้ว (เฉพาะที่ส่งออก)", "ลงทะเบียนแล้ว", "รอเรียกคิว", "กำลังเรียก", "กำลังปรึกษา", "เสร็จสิ้น", "ไม่มา"}}
	for _, k := range keys {
		g := groups[k]
		period := g.period.Start + " - " + g.period.End
		capacity := strconv.Itoa(g.period.Capacity)
		if g.period.Start == "" {
			period = "รอบเดิม " + g.period.ID
			capacity = ""
		}
		r := []string{g.rule.Date, g.rule.Level, subject(g.rule.Subject), period, g.rule.Mode, capacity, strconv.Itoa(g.total), strconv.Itoa(g.checked)}
		for _, status := range []string{"Booked", "Waiting", "Calling", "Serving", "Completed", "No-show"} {
			r = append(r, strconv.Itoa(g.states[status]))
		}
		summary = append(summary, r)
	}
	return []exportSheet{{"ผู้ลงทะเบียนทั้งหมด", rows, map[int]bool{13: true, 14: true}}, {"สรุปช่วงเวลา", summary, map[int]bool{5: true, 6: true, 7: true, 8: true, 9: true, 10: true, 11: true, 12: true, 13: true}}}, nil
}

type exportSheet struct {
	name    string
	rows    [][]string
	numeric map[int]bool
}

// Inline strings preserve Thai, leading-zero IDs and literal '=...' user input.
// No formulas, external links, credentials or private tracking tokens are emitted.
func xlsxWorkbook(sheets []exportSheet) ([]byte, error) {
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	write := func(name, body string) error {
		w, e := z.Create(name)
		if e != nil {
			return e
		}
		_, e = w.Write([]byte(body))
		return e
	}
	const ns = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	const rel = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	const header = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`
	content := header + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`
	workbook := header + `<workbook xmlns="` + ns + `" xmlns:r="` + rel + `"><sheets>`
	rels := header + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`
	for i, s := range sheets {
		id := strconv.Itoa(i + 1)
		content += `<Override PartName="/xl/worksheets/sheet` + id + `.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`
		workbook += `<sheet name="` + xmlText(s.name) + `" sheetId="` + id + `" r:id="rId` + id + `"/>`
		rels += `<Relationship Id="rId` + id + `" Type="` + rel + `/worksheet" Target="worksheets/sheet` + id + `.xml"/>`
		var body strings.Builder
		last := excelColumn(len(s.rows[0])) + strconv.Itoa(len(s.rows))
		body.WriteString(header + `<worksheet xmlns="` + ns + `"><dimension ref="A1:` + last + `"/><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><sheetFormatPr defaultRowHeight="30"/><cols>`)
		for c, title := range s.rows[0] {
			width := 24
			if strings.Contains(title, "คำถาม") || strings.Contains(title, "เหตุผล") || strings.Contains(title, "สัมมนารวม") {
				width = 48
			}
			if strings.Contains(title, "Email") {
				width = 34
			}
			n := strconv.Itoa(c + 1)
			fmt.Fprintf(&body, `<col min="%s" max="%s" width="%d" customWidth="1"/>`, n, n, width)
		}
		body.WriteString(`</cols><sheetData>`)
		for n, row := range s.rows {
			style := "1"
			if n == 0 {
				style = "2"
			} else if n%2 == 0 {
				style = "3"
			}
			if n == 0 {
				body.WriteString(`<row r="1" ht="45" customHeight="1">`)
			} else {
				fmt.Fprintf(&body, `<row r="%d">`, n+1)
			}
			for c, value := range row {
				if n > 0 && s.numeric[c] && value != "" {
					if _, err := strconv.Atoi(value); err != nil {
						return nil, err
					}
					fmt.Fprintf(&body, `<c r="%s%d" t="n" s="%s"><v>%s</v></c>`, excelColumn(c+1), n+1, style, value)
				} else {
					fmt.Fprintf(&body, `<c r="%s%d" t="inlineStr" s="%s"><is><t xml:space="preserve">%s</t></is></c>`, excelColumn(c+1), n+1, style, xmlText(value))
				}
			}
			body.WriteString(`</row>`)
		}
		body.WriteString(`</sheetData><autoFilter ref="A1:` + last + `"/></worksheet>`)
		if err := write("xl/worksheets/sheet"+id+".xml", body.String()); err != nil {
			return nil, err
		}
	}
	content += `</Types>`
	workbook += `</sheets></workbook>`
	rels += `<Relationship Id="styles" Type="` + rel + `/styles" Target="styles.xml"/></Relationships>`
	styles := header + `<styleSheet xmlns="` + ns + `"><fonts count="2"><font><sz val="11"/><name val="Tahoma"/></font><font><b/><sz val="11"/><color rgb="FFFFFFFF"/><name val="Tahoma"/></font></fonts><fills count="4"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF193047"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFF1F4F7"/><bgColor indexed="64"/></patternFill></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="4"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0" applyAlignment="1"><alignment vertical="top" wrapText="1"/></xf><xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyAlignment="1"><alignment vertical="top" wrapText="1"/></xf><xf numFmtId="0" fontId="0" fillId="3" borderId="0" xfId="0" applyAlignment="1"><alignment vertical="top" wrapText="1"/></xf></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`
	for _, file := range [][2]string{{"[Content_Types].xml", content}, {"_rels/.rels", header + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="` + rel + `/officeDocument" Target="xl/workbook.xml"/></Relationships>`}, {"xl/workbook.xml", workbook}, {"xl/_rels/workbook.xml.rels", rels}, {"xl/styles.xml", styles}} {
		if err := write(file[0], file[1]); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func excelColumn(n int) string {
	out := ""
	for n > 0 {
		n--
		out = string(rune('A'+n%26)) + out
		n /= 26
	}
	return out
}
func cleanExportText(value string) string {
	return strings.Map(func(r rune) rune {
		if r == 9 || r == 10 || r == 13 || (r >= 32 && r <= 0xD7FF) || (r >= 0xE000 && r <= 0xFFFD) || (r >= 0x10000 && r <= 0x10FFFF) {
			return r
		}
		return -1
	}, value)
}
func xmlText(value string) string {
	value = cleanExportText(value)
	var out bytes.Buffer
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}
