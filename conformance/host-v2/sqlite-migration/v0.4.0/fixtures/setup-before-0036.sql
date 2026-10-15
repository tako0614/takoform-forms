-- Test-only schema subset captured from sqlite_schema after the exact source
-- migrations 0001 through 0035 at the manifest source ref. Declarations are
-- preserved as SQLite stores them; this is trusted setup, not a migration.

CREATE TABLE actors (
  ap_id TEXT PRIMARY KEY,
  type TEXT NOT NULL DEFAULT 'Person',
  preferred_username TEXT NOT NULL UNIQUE,
  name TEXT,
  summary TEXT,
  icon_url TEXT,
  header_url TEXT,
  inbox TEXT NOT NULL,
  outbox TEXT NOT NULL,
  followers_url TEXT NOT NULL,
  following_url TEXT NOT NULL,
  public_key_pem TEXT NOT NULL,
  private_key_pem TEXT NOT NULL,
  takos_user_id TEXT UNIQUE,
  follower_count INTEGER DEFAULT 0,
  following_count INTEGER DEFAULT 0,
  post_count INTEGER DEFAULT 0,
  is_private INTEGER DEFAULT 0,
  role TEXT DEFAULT 'member',
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now')),
  deleted_at TEXT,
  owner_actor_ap_id TEXT
, fields_json TEXT NOT NULL DEFAULT '[]', also_known_as_json TEXT NOT NULL DEFAULT '[]', moved_to TEXT);

CREATE TABLE "activities" (
  ap_id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  actor_ap_id TEXT NOT NULL,
  object_ap_id TEXT,
  object_json TEXT,
  target_ap_id TEXT,
  raw_json TEXT NOT NULL,
  direction TEXT,
  processed INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE "inbox" (
  actor_ap_id TEXT NOT NULL,
  activity_ap_id TEXT NOT NULL REFERENCES activities(ap_id) ON DELETE CASCADE,
  read INTEGER DEFAULT 0,
  created_at TEXT DEFAULT (datetime('now')),
  PRIMARY KEY (actor_ap_id, activity_ap_id)
);

CREATE TABLE account_deletion_targets (
  actor_ap_id TEXT PRIMARY KEY NOT NULL,
  operation_id TEXT NOT NULL,
  public_key_pem TEXT NOT NULL,
  followers_url TEXT NOT NULL,
  delete_activity_ap_id TEXT NOT NULL,
  cascade_complete INTEGER NOT NULL DEFAULT 0 CHECK (cascade_complete IN (0, 1))
);

CREATE TABLE actor_write_assertions (
  id TEXT PRIMARY KEY NOT NULL,
  witness INTEGER NOT NULL,
  operation_id TEXT
);

CREATE TABLE notification_pushers (
  id TEXT PRIMARY KEY,
  actor_ap_id TEXT NOT NULL,
  product TEXT NOT NULL,
  scope TEXT,
  kind TEXT NOT NULL DEFAULT 'http',
  app_id TEXT NOT NULL,
  pushkey TEXT NOT NULL,
  pushkey_hash TEXT NOT NULL,
  app_display_name TEXT,
  device_display_name TEXT,
  profile_tag TEXT,
  lang TEXT,
  data_json TEXT NOT NULL DEFAULT '{}',
  gateway_url TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  last_seen_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE notification_push_jobs (
  id TEXT PRIMARY KEY,
  actor_ap_id TEXT NOT NULL,
  activity_ap_id TEXT NOT NULL,
  product TEXT,
  status TEXT NOT NULL DEFAULT 'pending',
  processing_token TEXT,
  attempts INTEGER NOT NULL DEFAULT 0,
  pending_pusher_ids_json TEXT,
  next_attempt_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  last_error TEXT,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  delivered_at TEXT
);

CREATE UNIQUE INDEX notification_push_jobs_actor_activity_idx
  ON notification_push_jobs(actor_ap_id, activity_ap_id);

CREATE INDEX notification_push_jobs_actor_idx
  ON notification_push_jobs(actor_ap_id);

CREATE INDEX notification_push_jobs_status_next_idx
  ON notification_push_jobs(status, next_attempt_at);

CREATE INDEX notification_push_jobs_terminal_retention_idx
  ON notification_push_jobs(status, updated_at);

CREATE INDEX notification_pushers_actor_product_idx
  ON notification_pushers(actor_ap_id, product);

CREATE UNIQUE INDEX notification_pushers_device_idx
  ON notification_pushers(product, app_id, pushkey_hash);

CREATE INDEX notification_pushers_last_seen_idx
  ON notification_pushers(last_seen_at);

CREATE TRIGGER account_deletion_notification_push_jobs_insert
BEFORE INSERT ON notification_push_jobs
WHEN EXISTS (
  SELECT 1 FROM account_deletion_targets AS target
  WHERE (target.actor_ap_id IN (NEW.actor_ap_id))
    AND NOT EXISTS (
      SELECT 1 FROM actor_write_assertions AS marker
      WHERE marker.operation_id = target.operation_id
    )
)
BEGIN
  SELECT RAISE(ABORT, 'account deletion reserved');
END;

CREATE TRIGGER account_deletion_notification_pushers_insert
BEFORE INSERT ON notification_pushers
WHEN EXISTS (
  SELECT 1 FROM account_deletion_targets AS target
  WHERE (target.actor_ap_id IN (NEW.actor_ap_id))
    AND NOT EXISTS (
      SELECT 1 FROM actor_write_assertions AS marker
      WHERE marker.operation_id = target.operation_id
    )
)
BEGIN
  SELECT RAISE(ABORT, 'account deletion reserved');
END;

CREATE TRIGGER account_deletion_notification_pushers_update
BEFORE UPDATE ON notification_pushers
WHEN EXISTS (
  SELECT 1 FROM account_deletion_targets AS target
  WHERE (target.actor_ap_id IN (OLD.actor_ap_id)
    OR target.actor_ap_id IN (NEW.actor_ap_id))
    AND NOT EXISTS (
      SELECT 1 FROM actor_write_assertions AS marker
      WHERE marker.operation_id = target.operation_id
    )
)
BEGIN
  SELECT RAISE(ABORT, 'account deletion reserved');
END;

CREATE TRIGGER notification_push_jobs_after_inbox_delete
AFTER DELETE ON inbox
BEGIN
  DELETE FROM notification_push_jobs
  WHERE actor_ap_id = OLD.actor_ap_id
    AND activity_ap_id = OLD.activity_ap_id
    AND status IN ('pending', 'queued', 'retry_wait');
END;

CREATE TRIGGER notification_push_jobs_after_inbox_insert
AFTER INSERT ON inbox
WHEN NEW.read = 0
BEGIN
  INSERT OR IGNORE INTO notification_push_jobs (
    id,
    actor_ap_id,
    activity_ap_id,
    product,
    status,
    attempts,
    next_attempt_at,
    created_at,
    updated_at
  ) VALUES (
    NEW.actor_ap_id || char(10) || NEW.activity_ap_id,
    NEW.actor_ap_id,
    NEW.activity_ap_id,
    NULL,
    'pending',
    0,
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
  );
END;
