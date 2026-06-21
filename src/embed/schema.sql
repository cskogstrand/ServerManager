-- Enable FKs
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS cache_track (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  key TEXT NOT NULL,
  config TEXT NOT NULL,
  name TEXT NOT NULL,
  desc TEXT,
  tags TEXT,
  country TEXT,
  city TEXT,
  length INTEGER,
  width INTEGER,
  pitboxes INTEGER,
  run TEXT,
  content_path TEXT,
  modified_at INTEGER
);

CREATE TABLE IF NOT EXISTS cache_car (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  key TEXT NOT NULL,
  name TEXT NOT NULL,
  brand TEXT,
  desc TEXT,
  tags TEXT,
  class TEXT,
  specs TEXT,
  torque TEXT,
  power TEXT,
  skins TEXT,
  content_path TEXT,
  modified_at INTEGER
);

CREATE TABLE IF NOT EXISTS cache_weather (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  key TEXT NOT NULL,
  name TEXT NOT NULL
);

-- Downscaled + re-encoded preview images, populated on demand by the
-- "Compress images" action. Served in preference to the full-size files on
-- disk/in smcontent.zip so the UI loads fast. Keyed so a row matches one
-- content item: kind ('car'|'track'), key (car/track folder), config (skin key
-- for cars, layout config for tracks; '' when none), variant ('preview' for
-- cars; 'preview'|'outline'|'map' for tracks).
CREATE TABLE IF NOT EXISTS cache_image (
  kind TEXT NOT NULL,
  key TEXT NOT NULL,
  config TEXT NOT NULL DEFAULT '',
  variant TEXT NOT NULL,
  content_type TEXT NOT NULL,
  data BLOB NOT NULL,
  PRIMARY KEY (kind, key, config, variant)
);

CREATE TABLE IF NOT EXISTS user_config (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT,
  password TEXT,
  admin_password TEXT,
  register_to_lobby INTEGER,
  locked_entry_list INTEGER,
  result_screen_time INTEGER,
  udp_port INTEGER,
  tcp_port INTEGER,
  http_port INTEGER,
  client_send_interval INTEGER,
  num_threads INTEGER,
  max_clients INTEGER,
  welcome_message TEXT,

  append_eventname INTEGER,
  append_modlinks INTEGER,
  mod_download_url TEXT,
  server_engine TEXT DEFAULT 'kunos',
  as_relax_checksums INTEGER DEFAULT 0,
  auto_start_server INTEGER DEFAULT 0,

  install_path TEXT,
  csp_required INTEGER,
  csp_phycars INTEGER,
  csp_phytracks INTEGER,
  csp_hidepit INTEGER,
  csp_version INTEGER,

  secret_key TEXT,
  cfg_filled INTEGER DEFAULT 0,
  mod_filled INTEGER DEFAULT 0,

  -- Drift-spike auto-capture tuning (see drivercapture.go).
  capture_enabled INTEGER NOT NULL DEFAULT 1,
  capture_screenshots INTEGER NOT NULL DEFAULT 1,
  capture_clips INTEGER NOT NULL DEFAULT 1,
  capture_trigger_score INTEGER NOT NULL DEFAULT 2500,
  capture_clip_seconds INTEGER NOT NULL DEFAULT 14,
  capture_cooldown_seconds INTEGER NOT NULL DEFAULT 45,
  capture_max_per_session INTEGER NOT NULL DEFAULT 12
);

CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT,
  password TEXT,

  measurement_unit INTEGER,
  temp_unit INTEGER
);

CREATE TABLE IF NOT EXISTS user_difficulty (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  abs_allowed INTEGER,
  tc_allowed INTEGER,
  stability_allowed INTEGER,
  autoclutch_allowed INTEGER,
  tyre_blankets_allowed INTEGER,
  force_virtual_mirror INTEGER,
  fuel_rate INTEGER,
  damage_multiplier INTEGER,
  tyre_wear_rate INTEGER,
  allowed_tyres_out INTEGER,
  max_ballast_kg INTEGER,
  start_rule INTEGER,
  race_gas_penality_disabled INTEGER,
  dynamic_track INTEGER,
  dynamic_track_preset INTEGER,
  session_start INTEGER,
  randomness INTEGER,
  session_transfer INTEGER,
  lap_gain INTEGER,
  kick_quorum INTEGER,
  vote_duration INTEGER,
  voting_quorum INTEGER,
  blacklist_mode INTEGER,
  max_contacts_per_km INTEGER,

  filled INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS user_time (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  time TEXT,
  time_of_day_multi INTEGER,
  csp_enabled INTEGER,

  filled INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS user_time_weather (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_time_id INTEGER NOT NULL,
  graphics TEXT,
  base_temperature_ambient INTEGER,
  base_temperature_road INTEGER,
  variation_ambient INTEGER,
  variation_road INTEGER,
  wind_base_speed_min INTEGER,
  wind_base_speed_max INTEGER,
  wind_base_direction INTEGER,
  wind_variation_direction INTEGER,
  csp_time TEXT,
  csp_time_of_day_multi INTEGER,
  csp_date TEXT,

  FOREIGN KEY (user_time_id) REFERENCES user_time(id) ON DELETE RESTRICT,
  FOREIGN KEY (graphics) REFERENCES cache_weather(key) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS user_session (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  booking_enabled INTEGER,
  booking_time INTEGER,
  practice_enabled INTEGER,
  practice_time INTEGER,
  practice_is_open INTEGER,
  qualify_enabled INTEGER,
  qualify_time INTEGER,
  qualify_is_open INTEGER,
  qualify_max_wait_perc INTEGER,
  race_enabled INTEGER,
  race_time INTEGER,
  race_extra_lap INTEGER,
  race_over_time INTEGER,
  race_wait_time INTEGER,
  race_is_open INTEGER,
  reversed_grid_positions INTEGER,
  race_pit_window_start INTEGER,
  race_pit_window_end INTEGER,

  filled INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS user_class (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,

  filled INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS user_class_entry (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_class_id INTEGER NOT NULL,
  cache_car_key TEXT NOT NULL,
  skin_key TEXT NOT NULL,
  ballast INTEGER,
  car_count INTEGER NOT NULL DEFAULT 1,

  FOREIGN KEY (user_class_id) REFERENCES user_class(id) ON DELETE RESTRICT,
  FOREIGN KEY (cache_car_key) REFERENCES cache_car(key) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS user_event_category (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,

  filled INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS user_event (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  event_category_id INTEGER NOT NULL,
  cache_track_key TEXT NOT NULL,
  cache_track_config TEXT NOT NULL,
  difficulty_id INTEGER NOT NULL,
  session_id INTEGER NOT NULL,
  class_id INTEGER NOT NULL,
  time_id INTEGER NOT NULL,

  name TEXT,
  race_laps INTEGER,
  strategy INTEGER,

  FOREIGN KEY (event_category_id) REFERENCES user_event_category(id) ON DELETE RESTRICT,
  FOREIGN KEY (cache_track_key, cache_track_config) REFERENCES cache_track(key, config) ON DELETE RESTRICT,
  FOREIGN KEY (difficulty_id) REFERENCES user_difficulty(id) ON DELETE RESTRICT,
  FOREIGN KEY (session_id) REFERENCES user_session(id) ON DELETE RESTRICT,
  FOREIGN KEY (class_id) REFERENCES user_class(id) ON DELETE RESTRICT,
  FOREIGN KEY (time_id) REFERENCES user_time(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS server_event (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_event_id INTEGER NOT NULL,

  servercfg TEXT,
  entrylist TEXT,

  started_at INTEGER,
  finished INTEGER DEFAULT 0,
  orderby INTEGER,
  instance_id INTEGER NOT NULL DEFAULT 1,

  FOREIGN KEY (user_event_id) REFERENCES user_event(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS server_instance (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  udp_port INTEGER NOT NULL,
  tcp_port INTEGER NOT NULL,
  http_port INTEGER NOT NULL,
  plugin_port INTEGER NOT NULL,
  plugin_listen_port INTEGER NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  stream_enabled INTEGER NOT NULL DEFAULT 0,
  stream_embed_url TEXT,
  stream_status_url TEXT,
  spectator_enabled INTEGER NOT NULL DEFAULT 0,
  spectator_driver_name TEXT,
  spectator_guid TEXT,
  spectator_car_key TEXT,
  spectator_skin_key TEXT,
  start_on_boot INTEGER NOT NULL DEFAULT 0,
  drift_score_enabled INTEGER NOT NULL DEFAULT 0,
  allow_wrong_way INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS driver_stream (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  driver_guid TEXT NOT NULL UNIQUE,
  display_name TEXT,
  enabled INTEGER NOT NULL DEFAULT 1,
  stream_embed_url TEXT NOT NULL,
  stream_status_url TEXT,
  stream_capture_url TEXT
);

-- Driver Stats persistence. Live driver/lap/drift state is session-scoped and
-- wiped each session; these tables keep the history the Driver Stats pages
-- aggregate. No FK constraints on purpose: telemetry writes must never fail on
-- a missing parent row (the driver row is upserted on join, but analytics
-- inserts should degrade gracefully rather than drop events).
CREATE TABLE IF NOT EXISTS driver (
  guid TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  first_seen INTEGER NOT NULL,
  last_seen INTEGER NOT NULL,
  avatar_path TEXT
);

CREATE TABLE IF NOT EXISTS driver_session (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  driver_guid TEXT NOT NULL,
  instance_id INTEGER NOT NULL,
  session_type INTEGER NOT NULL,
  car_key TEXT,
  skin_key TEXT,
  track_key TEXT,
  track_config TEXT,
  started_at INTEGER NOT NULL,
  ended_at INTEGER NOT NULL,
  laps INTEGER NOT NULL DEFAULT 0,
  best_lap_ms INTEGER NOT NULL DEFAULT 0,
  finish_pos INTEGER,
  entrants INTEGER,
  drift_best INTEGER NOT NULL DEFAULT 0,
  -- Optional override: attribute this row to a guest_driver (see below) instead
  -- of the GUID's own name in leaderboards. NULL = use the driver's name.
  guest_driver_id INTEGER,
  -- The driver_connection (one connect→disconnect span) this AC-session segment
  -- belongs to. A single connection can hold several segments (practice/qualify/
  -- race). NULL on rows written before connections existed.
  connection_id INTEGER
);

CREATE TABLE IF NOT EXISTS driver_drift_run (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  driver_guid TEXT NOT NULL,
  instance_id INTEGER NOT NULL,
  track_key TEXT,
  track_config TEXT,
  car_key TEXT,
  score INTEGER NOT NULL,
  ended_at INTEGER NOT NULL,
  -- See driver_session.guest_driver_id. NULL = use the driver's own name.
  guest_driver_id INTEGER,
  -- driver_connection this run happened in (NULL on legacy rows).
  connection_id INTEGER
);

-- Guest drivers: a roster of real people who may share one Assetto Corsa
-- account (GUID). A leaderboard row (drift run / timed lap) or a live, connected
-- car can be attributed to one of these instead of the GUID's own name, so
-- "who actually drove" is recorded even when several people use one account.
CREATE TABLE IF NOT EXISTS guest_driver (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  notes TEXT,
  avatar_path TEXT,
  created_at INTEGER NOT NULL
);

-- Auto-captured stream highlights (screenshots/clips on big drift spikes). The
-- capture worker is a planned follow-up (see docs/driver-stats.md); the table +
-- read path ship now so the contract is complete.
CREATE TABLE IF NOT EXISTS driver_media (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  driver_guid TEXT NOT NULL,
  session_id INTEGER,
  kind TEXT NOT NULL,
  path TEXT NOT NULL,
  caption TEXT,
  captured_at INTEGER NOT NULL,
  duration_s INTEGER,
  trigger_score INTEGER,
  trigger_delta INTEGER,
  drift_run_id INTEGER,
  -- driver_connection this capture happened in (NULL on legacy/manual rows).
  connection_id INTEGER
);

-- A driver "session": one continuous connection, from connect to disconnect. It
-- groups the per-AC-session driver_session segments, laps, drift runs and media
-- that happened while the driver was connected, and can carry searchable tags.
-- left_at is NULL while the driver is still connected.
CREATE TABLE IF NOT EXISTS driver_connection (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  driver_guid TEXT NOT NULL,
  instance_id INTEGER NOT NULL,
  joined_at INTEGER NOT NULL,
  left_at INTEGER,
  car_key TEXT,
  skin_key TEXT,
  track_key TEXT,
  track_config TEXT,
  guest_driver_id INTEGER
);

-- One completed lap, attributed to the connection it was set in. session_type is
-- the AC session (0 booking, 1 practice, 2 qualify, 3 race) at the time.
CREATE TABLE IF NOT EXISTS driver_lap (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  connection_id INTEGER NOT NULL,
  driver_guid TEXT NOT NULL,
  instance_id INTEGER NOT NULL,
  session_type INTEGER NOT NULL,
  track_key TEXT,
  track_config TEXT,
  car_key TEXT,
  lap_number INTEGER NOT NULL,
  laptime_ms INTEGER NOT NULL,
  cuts INTEGER NOT NULL DEFAULT 0,
  recorded_at INTEGER NOT NULL
);

-- Free-text tags on a connection, so a session can be found again later. Several
-- tags per connection; a tag is unique within its connection.
CREATE TABLE IF NOT EXISTS driver_session_tag (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  connection_id INTEGER NOT NULL,
  tag TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  UNIQUE(connection_id, tag)
);




-- INDEXES

CREATE INDEX IF NOT EXISTS idx_driver_session_guid ON driver_session (driver_guid);
CREATE INDEX IF NOT EXISTS idx_driver_drift_run_guid ON driver_drift_run (driver_guid);
CREATE INDEX IF NOT EXISTS idx_driver_media_guid ON driver_media (driver_guid);
CREATE INDEX IF NOT EXISTS idx_driver_connection_guid ON driver_connection (driver_guid);
CREATE INDEX IF NOT EXISTS idx_driver_lap_conn ON driver_lap (connection_id);
CREATE INDEX IF NOT EXISTS idx_session_tag_tag ON driver_session_tag (tag);
CREATE INDEX IF NOT EXISTS idx_session_tag_conn ON driver_session_tag (connection_id);
-- NOTE: indexes on driver_session/driver_drift_run/driver_media.connection_id are
-- created in dbaccess.go AFTER the ensureColumn migrations add that column —
-- this file runs before those ALTERs, so the column may not exist here yet.

CREATE UNIQUE INDEX IF NOT EXISTS idx_cache_track_key_config
ON cache_track (key, config);

CREATE UNIQUE INDEX IF NOT EXISTS idx_cache_car_key
ON cache_car (key);

CREATE UNIQUE INDEX IF NOT EXISTS idx_cache_weather_key
ON cache_weather (key);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_name
ON users (name);


-- DEFAULT VALUES
INSERT OR IGNORE INTO user_config (id, name, udp_port, tcp_port, http_port, client_send_interval, num_threads, secret_key) VALUES (1, 'SM Server', 9600, 9600, 8081, 18, 2, hex(randomblob(16)));

-- Default instance inherits the ports historically stored in user_config
INSERT OR IGNORE INTO server_instance (id, name, udp_port, tcp_port, http_port, plugin_port, plugin_listen_port)
SELECT 1, COALESCE(name, 'SM Server'), COALESCE(udp_port, 9600), COALESCE(tcp_port, 9600), COALESCE(http_port, 8081), 5000, 5001
FROM user_config WHERE id = 1;

-- DEFAULT USERNAME admin PASSWORD admin
INSERT OR IGNORE INTO users (id, name, password) VALUES (1, 'admin', '$2a$08$BvgMQY6H60BhcK9wM79RBu9IlURIP26BWYcCiWJjs06L1yEdkUif2');
