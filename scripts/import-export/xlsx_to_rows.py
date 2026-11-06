#!/usr/bin/env python3
"""Convert a Staff Excel export into JSON rows for the local importer.

Usage:
  python3 xlsx_to_rows.py <export.xlsx> <event-config.json> <rows.json>

Reads the "ผู้ลงทะเบียนทั้งหมด" sheet, maps exported Thai statuses back to API
statuses and resolves each "time range" to the configured period ID. No data is
written to any database here; the JSON is consumed by
`backend/cmd/importregistrations`.
"""
import json
import sys
from openpyxl import load_workbook

STATUS = {
    'ลงทะเบียนแล้ว': 'Booked',
    'รอเรียกคิว': 'Waiting',
    'กำลังเรียก': 'Calling',
    'กำลังปรึกษา': 'Serving',
    'เสร็จสิ้น': 'Completed',
    'ไม่มา': 'No-show',
    'ยกเลิก': 'Cancelled',
}


def text(value):
    if value is None:
        return ''
    value = str(value).strip()
    return '' if value == '-' else value


def number(value):
    value = text(value)
    return int(value) if value.isdigit() else 0


def flag(value):
    return text(value) in ('ใช่', 'true', 'True', '1')


def main():
    if len(sys.argv) != 4:
        sys.exit(__doc__.strip())
    xlsx_path, config_path, out_path = sys.argv[1:4]

    config = json.load(open(config_path, encoding='utf-8'))
    periods = {}
    for rule in config['rules']:
        for period in rule['periods']:
            key = (rule['level'], rule['subject'], rule['date'], f"{period['start']} - {period['end']}")
            if key in periods and periods[key] != period['id']:
                raise SystemExit(f'ambiguous period mapping for {key}')
            periods[key] = period['id']

    workbook = load_workbook(xlsx_path, read_only=True, data_only=True)
    sheet = workbook['ผู้ลงทะเบียนทั้งหมด']
    rows = list(sheet.iter_rows(values_only=True))
    header = [text(h) for h in rows[0]]
    column = {name: index for index, name in enumerate(header)}
    checkin_header = next((h for h in header if h.startswith('เวลาเช็กอิน')), None)

    def get(row, name):
        return row[column[name]] if name in column else None

    out = []
    for row_number, row in enumerate(rows[1:], start=2):
        if not any(row):
            continue
        booking_id = text(get(row, 'รหัสการจอง'))
        level = text(get(row, 'ระดับ'))
        subject = {'Math + Verbal': 'Combined'}.get(text(get(row, 'วิชาที่จอง')), text(get(row, 'วิชาที่จอง')))
        date = text(get(row, 'วันที่ Consult'))
        time_range = text(get(row, 'ช่วงเวลา Consult'))
        period_id = periods.get((level, subject, date, time_range))
        if not booking_id or not period_id:
            raise SystemExit(f'row {row_number}: cannot map booking/period (level={level}, subject={subject}, date={date}, time={time_range})')
        raw_status = text(get(row, 'สถานะ'))
        status = STATUS.get(raw_status)
        if not status:
            raise SystemExit(f'row {row_number}: unknown status {raw_status!r}')

        orders = {}
        for stream in ('Math', 'Verbal'):
            value = number(get(row, f'ลำดับ {stream}'))
            if value:
                orders[stream] = value
        stages = {}
        for stream in ('Math', 'Verbal'):
            stage = {}
            for api_field, export_field in (('calledAt', 'เรียก'), ('startedAt', 'เริ่ม'), ('endedAt', 'จบ')):
                value = text(get(row, f'{stream} {export_field}'))
                if value:
                    stage[api_field] = value
            if stage:
                stages[stream] = stage

        out.append({
            'id': booking_id,
            'requestKey': booking_id,
            'email': text(get(row, 'Email ผู้ปกครอง')),
            'wcode': text(get(row, 'Wcode')),
            'nickname': text(get(row, 'ชื่อเล่นนักเรียน')),
            'school': text(get(row, 'โรงเรียน')),
            'level': level,
            'subject': subject,
            'date': date,
            'periodId': period_id,
            'questions': [text(get(row, f'คำถาม {n}')) for n in (1, 2, 3)],
            'seminarConfirmed': flag(get(row, 'ยืนยันสัมมนา Beginner')),
            'status': status,
            'currentSubject': text(get(row, 'วิชาปัจจุบัน')),
            'orders': orders,
            'queueNo': text(get(row, 'รหัสคิว')),
            'checkedInAt': text(get(row, checkin_header)) if checkin_header else '',
            'stages': stages,
            'mode': text(get(row, 'รูปแบบ')),
            'counter': text(get(row, 'ห้อง / เคาน์เตอร์')),
            'staff': text(get(row, 'Staff')),
            'createdAt': text(get(row, 'เวลาลงทะเบียน')),
            'updatedAt': text(get(row, 'แก้ไขล่าสุด')),
            'completedAt': text(get(row, 'เวลาจบเคส')),
            'cancelledAt': text(get(row, 'เวลายกเลิก')),
            'cancellationReason': text(get(row, 'เหตุผลยกเลิก')),
            'archivedAt': text(get(row, 'เวลาเก็บรายการ')),
            'archivedBy': text(get(row, 'ผู้เก็บรายการ')),
            'emailStatus': text(get(row, 'สถานะอีเมล')) or 'Sent',
        })

    json.dump(out, open(out_path, 'w', encoding='utf-8'), ensure_ascii=False, indent=1)
    statuses = {}
    subjects = {}
    for item in out:
        statuses[item['status']] = statuses.get(item['status'], 0) + 1
        subjects[item['subject']] = subjects.get(item['subject'], 0) + 1
    print(f'converted {len(out)} registrations')
    print('status:', statuses)
    print('subject:', subjects)
    missing = [item['id'] for item in out if not item['email'] or not item['nickname'] or not item['school']]
    print('rows missing required contact fields:', len(missing))


if __name__ == '__main__':
    main()
