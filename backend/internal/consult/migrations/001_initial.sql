CREATE TABLE settings(name TEXT PRIMARY KEY,value TEXT NOT NULL);
CREATE TABLE meta(id SMALLINT PRIMARY KEY CHECK(id=1),revision BIGINT NOT NULL);
INSERT INTO meta VALUES(1,0);
CREATE TABLE periods(
 event_id TEXT NOT NULL,level TEXT NOT NULL,subject TEXT NOT NULL,date TEXT NOT NULL,period_id TEXT NOT NULL,
 start TEXT NOT NULL,"end" TEXT NOT NULL,capacity INTEGER NOT NULL CHECK(capacity>0),mode TEXT NOT NULL,
 PRIMARY KEY(event_id,level,subject,date,period_id)
);
CREATE TABLE bookings(
 id TEXT PRIMARY KEY,event_id TEXT NOT NULL,level TEXT NOT NULL,subject TEXT NOT NULL,date TEXT NOT NULL,period_id TEXT NOT NULL,
 current_subject TEXT NOT NULL,status TEXT NOT NULL CHECK(status IN ('Booked','Waiting','Calling','Serving','Completed','No-show','Cancelled')),
 queue_no TEXT,request_key TEXT NOT NULL,payload_hash TEXT NOT NULL,token_hash TEXT NOT NULL UNIQUE,token_cipher TEXT NOT NULL,data JSONB NOT NULL,
 UNIQUE(event_id,request_key),UNIQUE(event_id,queue_no),
 FOREIGN KEY(event_id,level,subject,date,period_id) REFERENCES periods(event_id,level,subject,date,period_id),
 CHECK(COALESCE(data->>'id'=id AND data->>'eventId'=event_id AND data->>'level'=level AND data->>'subject'=subject AND data->>'date'=date AND data->>'periodId'=period_id AND data->>'currentSubject'=current_subject AND data->>'status'=status AND COALESCE(data->>'queueNo','')=COALESCE(queue_no,''),false))
);
CREATE INDEX capacity_scope ON bookings(event_id,level,subject,date,period_id,status);
CREATE UNIQUE INDEX active_stream ON bookings(event_id,level,date,current_subject) WHERE status IN ('Calling','Serving');
CREATE TABLE sequences(scope TEXT PRIMARY KEY,value BIGINT NOT NULL);
CREATE TABLE audit(id BIGSERIAL PRIMARY KEY,booking_id TEXT NOT NULL,action TEXT NOT NULL,actor TEXT NOT NULL,previous TEXT NOT NULL,next TEXT NOT NULL,at TEXT NOT NULL);
CREATE TABLE users(username TEXT PRIMARY KEY,password_hash TEXT NOT NULL,role TEXT NOT NULL CHECK(role IN ('admin','operator')));
CREATE TABLE sessions(token_hash TEXT PRIMARY KEY,username TEXT NOT NULL REFERENCES users(username),expires_at BIGINT NOT NULL);
CREATE INDEX session_expiry ON sessions(expires_at);
CREATE TABLE outbox(booking_id TEXT PRIMARY KEY REFERENCES bookings(id),state TEXT NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,next_at BIGINT NOT NULL DEFAULT 0,lease_at BIGINT NOT NULL DEFAULT 0,last_error TEXT NOT NULL DEFAULT '');
CREATE INDEX pending_notifications ON outbox(state,next_at);

-- The same guard row is locked by application mutations and this trigger.
-- At READ COMMITTED, the capacity count runs after acquiring the lock.
CREATE FUNCTION enforce_booking_capacity() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE maximum INTEGER; occupied INTEGER;
BEGIN
 PERFORM id FROM meta WHERE id=1 FOR UPDATE;
 IF NEW.status='Cancelled' THEN RETURN NEW; END IF;
 SELECT capacity INTO maximum FROM periods WHERE event_id=NEW.event_id AND level=NEW.level AND subject=NEW.subject AND date=NEW.date AND period_id=NEW.period_id;
 IF maximum IS NULL THEN RAISE EXCEPTION 'unknown period' USING ERRCODE='23503'; END IF;
 SELECT COUNT(*) INTO occupied FROM bookings WHERE id<>NEW.id AND event_id=NEW.event_id AND level=NEW.level AND subject=NEW.subject AND date=NEW.date AND period_id=NEW.period_id AND status<>'Cancelled';
 IF occupied>=maximum THEN RAISE EXCEPTION 'period capacity exceeded' USING ERRCODE='23514',CONSTRAINT='booking_capacity'; END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER capacity_guard BEFORE INSERT OR UPDATE OF status,level,subject,date,period_id ON bookings FOR EACH ROW EXECUTE FUNCTION enforce_booking_capacity();
