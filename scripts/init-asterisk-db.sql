-- =============================================================================
-- init-asterisk-db.sql  — Optional: Asterisk CDR (Call Detail Records) + CEL
-- (Channel Event Logging) tables when ODBC CDR backend is enabled in
-- docker-compose via ASTERISK_CDR_ODBC_DSN.
--
-- HOW TO USE:
--   1. Enable ASTERISK_CDR_ODBC_DSN in .env.docker.example
--   2. Configure /etc/asterisk/res_odbc.conf + cdr_adaptive_odbc.conf
--   3. Mount this file into the postgres init directory or run by hand:
--      docker exec -i dongdo_postgres psql -U postgres -d dongdo_cs \\
--          < scripts/init-asterisk-db.sql
--
-- All schema is namespaced as `asterisk.*` so it doesn't collide with the
-- application schema.
-- =============================================================================

CREATE SCHEMA IF NOT EXISTS asterisk;

-- ----------------------------------------------------------------------------
-- asterisk.cdr  — Call Detail Records (one row per completed call)
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS asterisk.cdr (
    id                  BIGSERIAL PRIMARY KEY,

    -- pjsip endpoint / trunk identifiers
    accountcode         VARCHAR(80),
    clid                VARCHAR(80),
    src                 VARCHAR(80),
    dst                 VARCHAR(80),
    dcontext            VARCHAR(80),
    lastapp             VARCHAR(80),
    lastdata            TEXT,

    -- Timing
    start_time          TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    answer_time         TIMESTAMP WITHOUT TIME ZONE,
    end_time            TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    duration            INTEGER NOT NULL,    -- total ring time incl. talk
    billsec             INTEGER NOT NULL,    -- answered duration (seconds)
    disposition         VARCHAR(32) NOT NULL, -- ANSWERED / NO ANSWER / BUSY / FAILED
    amaflags            VARCHAR(16),

    -- Recording files
    recording_path      TEXT,

    -- Asterisk metadata
    uniqueid            VARCHAR(150),       -- matching channel UID
    sequence            VARCHAR(32),
    userfield           VARCHAR(255),
    dstchannel          VARCHAR(80),
    peer                VARCHAR(80),

    -- Bookkeeping
    ingest_id           UUID,
    ingested_at         TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),

    CONSTRAINT cdr_disposition_check
        CHECK (disposition IN ('ANSWERED','NO ANSWER','BUSY','FAILED','CONGESTION'))
);

CREATE INDEX IF NOT EXISTS cdr_uniqueid_idx  ON asterisk.cdr (uniqueid);
CREATE INDEX IF NOT EXISTS cdr_start_idx     ON asterisk.cdr (start_time DESC);
CREATE INDEX IF NOT EXISTS cdr_src_idx       ON asterisk.cdr (src);
CREATE INDEX IF NOT EXISTS cdr_dst_idx       ON asterisk.cdr (dst);
CREATE INDEX IF NOT EXISTS cdr_disp_idx      ON asterisk.cdr (disposition);
CREATE INDEX IF NOT EXISTS cdr_accountcode_idx ON asterisk.cdr (accountcode);

-- ----------------------------------------------------------------------------
-- asterisk.cel  — Channel Event Logging (per-event; much higher volume)
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS asterisk.cel (
    id                  BIGSERIAL PRIMARY KEY,

    eventtype           VARCHAR(64) NOT NULL,   -- CHAN_START, ANSWER, BRIDGE_ENTER, etc.
    eventname           VARCHAR(80),
    eventtime           TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    uniqueid            VARCHAR(150),
    linkedid            VARCHAR(150),

    context             VARCHAR(80),
    exten               VARCHAR(80),
    priority            VARCHAR(16),
    channel             VARCHAR(80),
    channelstate        VARCHAR(32),
    channelstatedesc    VARCHAR(80),

    calleridnum         VARCHAR(80),
    calleridname        VARCHAR(80),
    connectedlinenum    VARCHAR(80),
    connectedlinename   VARCHAR(80),

    application         VARCHAR(80),
    appdata             TEXT,
    peeraccount         VARCHAR(80),

    extra               VARCHAR(255),

    ingest_id           UUID,
    ingested_at         TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS cel_uniqueid_idx    ON asterisk.cel (uniqueid);
CREATE INDEX IF NOT EXISTS cel_linkedid_idx    ON asterisk.cel (linkedid);
CREATE INDEX IF NOT EXISTS cel_eventtime_idx   ON asterisk.cel (eventtime DESC);
CREATE INDEX IF NOT EXISTS cel_eventtype_idx   ON asterisk.cel (eventtype);

-- ----------------------------------------------------------------------------
-- asterisk.queues_log  — Optional: queue member / state transitions
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS asterisk.queues_log (
    id                  BIGSERIAL PRIMARY KEY,
    event_time          TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    queue               VARCHAR(80),
    uniqueid            VARCHAR(150),
    calleridnum         VARCHAR(80),
    membername          VARCHAR(80),
    position            INTEGER,
    event               VARCHAR(64)                -- JOIN, LEAVE, ABANDON, etc.
);
CREATE INDEX IF NOT EXISTS ql_q_idx  ON asterisk.queues_log (queue, event_time DESC);
CREATE INDEX IF NOT EXISTS ql_e_idx  ON asterisk.queues_log (event);

COMMENT ON SCHEMA asterisk IS
    'Asterisk-internal tables: CDR (call detail), CEL (event log), queues_log.';