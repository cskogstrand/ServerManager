package main

type Users struct {
	Name            *string `form:"name" json:"name"`
	Password        *string `form:"password" json:"password,omitempty"`
	MeasurementUnit *int    `form:"measurement_unit" json:"measurement_unit"`
	TempUnit        *int    `form:"temp_unit" json:"temp_unit"`
	Role            *string `form:"role" json:"role"`
}

// Role capabilities: admin = everything; steward = operate running servers and
// queues; viewer = read-only.
const (
	roleAdmin   = "admin"
	roleSteward = "steward"
	roleViewer  = "viewer"
)

type UserConfig struct {
	Name               *string `form:"name" json:"name"`
	Password           *string `form:"password" json:"password"`
	AdminPassword      *string `form:"admin_password" json:"admin_password"`
	RegisterToLobby    *int    `form:"register_to_lobby" json:"register_to_lobby"`
	LockedEntryList    *int    `form:"locked_entry_list" json:"locked_entry_list"`
	ResultScreenTime   *int    `form:"result_screen_time" json:"result_screen_time"`
	UdpPort            *int    `form:"udp_port" json:"udp_port"`
	TcpPort            *int    `form:"tcp_port" json:"tcp_port"`
	HttpPort           *int    `form:"http_port" json:"http_port"`
	ClientSendInterval *int    `form:"client_send_interval" json:"client_send_interval"`
	NumThreads         *int    `form:"num_threads" json:"num_threads"`
	MaxClients         *int    `form:"max_clients" json:"max_clients"`
	WelcomeMessage     *string `form:"welcome_message" json:"welcome_message"`
	AppendEventname    *int    `form:"append_eventname" json:"append_eventname"`
	AppendModlinks     *int    `form:"append_modlinks" json:"append_modlinks"`
	ModDownloadUrl     *string `form:"mod_download_url" json:"mod_download_url"`
	ServerEngine       *string `form:"server_engine" json:"server_engine"`
	AsRelaxChecksums   *int    `form:"as_relax_checksums" json:"as_relax_checksums"`
	AutoStartServer    *int    `form:"auto_start_server" json:"auto_start_server"`
	InstallPath        *string `form:"install_path" json:"install_path"`
	CspRequired        *int    `form:"csp_required" json:"csp_required"`
	CspVersion         *int    `form:"csp_version" json:"csp_version"`
	CspPhycars         *int    `form:"csp_phycars" json:"csp_phycars"`
	CspPhytracks       *int    `form:"csp_phytracks" json:"csp_phytracks"`
	CspHidepit         *int    `form:"csp_hidepit" json:"csp_hidepit"`
	CfgFilled          *int    `json:"cfg_filled"`
	ModFilled          *int    `json:"mod_filled"`
	SecretKey          *string `json:"-"`
}

type ServerEvent struct {
	Id         *int
	UserEvent  UserEvent
	ServerCfg  *string
	EntryList  *string
	StartedAt  *int64
	Finished   *int
	InstanceId *int `json:"instance_id"`
}

type ServerInstance struct {
	Id               *int    `json:"id"`
	Name             *string `json:"name" form:"name"`
	UdpPort          *int    `json:"udp_port" form:"udp_port"`
	TcpPort          *int    `json:"tcp_port" form:"tcp_port"`
	HttpPort         *int    `json:"http_port" form:"http_port"`
	PluginPort       *int    `json:"plugin_port" form:"plugin_port"`
	PluginListenPort *int    `json:"plugin_listen_port" form:"plugin_listen_port"`
	Enabled          *int    `json:"enabled" form:"enabled"`
	StreamEnabled    *int    `json:"stream_enabled" form:"stream_enabled"`
	StreamEmbedUrl   *string `json:"stream_embed_url" form:"stream_embed_url"`
	StreamStatusUrl  *string `json:"stream_status_url" form:"stream_status_url"`
	SpectatorEnabled  *int    `json:"spectator_enabled" form:"spectator_enabled"`
	SpectatorName     *string `json:"spectator_driver_name" form:"spectator_driver_name"`
	SpectatorGuid     *string `json:"spectator_guid" form:"spectator_guid"`
	SpectatorCarKey   *string `json:"spectator_car_key" form:"spectator_car_key"`
	SpectatorSkinKey  *string `json:"spectator_skin_key" form:"spectator_skin_key"`
	// RunMode is "manual_queue" (default) or "repeat_event". In repeat mode the
	// instance re-applies RepeatEventId every time a session ends instead of
	// advancing the manual queue.
	RunMode       *string `json:"run_mode" form:"run_mode"`
	RepeatEventId *int    `json:"repeat_event_id" form:"repeat_event_id"`
	// ScheduledStart is a unix timestamp; the scheduler starts the instance at
	// or after this time, then clears it. nil/0 means no schedule.
	ScheduledStart *int64 `json:"scheduled_start" form:"scheduled_start"`
	// StartOnBoot auto-starts this instance from its queue when SM launches.
	StartOnBoot *int `json:"start_on_boot" form:"start_on_boot"`
	// DriftScoreEnabled serves the drift-score CSP Lua HUD to clients. Only has
	// an effect on the AssettoServer engine (vanilla acServer has no CSP-script
	// delivery channel).
	DriftScoreEnabled *int `json:"drift_score_enabled" form:"drift_score_enabled"`
}

type DriverStream struct {
	Id              *int    `json:"id"`
	DriverGuid      *string `json:"driver_guid" form:"driver_guid"`
	DisplayName     *string `json:"display_name" form:"display_name"`
	Enabled         *int    `json:"enabled" form:"enabled"`
	StreamEmbedUrl  *string `json:"stream_embed_url" form:"stream_embed_url"`
	StreamStatusUrl *string `json:"stream_status_url" form:"stream_status_url"`
}

const (
	runModeManualQueue = "manual_queue"
	runModeRepeatEvent = "repeat_event"
)

// Dedicated-server engine. Kunos is the stock acServer; AssettoServer is the
// drop-in replacement that natively serves the Content Manager /api/details
// "content" field (the "Install missing content" button).
const (
	engineKunos         = "kunos"
	engineAssettoServer = "assettoserver"
)

type UserEvent struct {
	Id                *int    `form:"id"`
	Name              *string `form:"name" json:"name"`
	EventCategoryId   *int
	CategoryName      *string
	RaceLaps          *int `form:"race_laps" json:"race_laps,string"`
	Strategy          *int `form:"strategy" json:"strategy,string"`
	TrackName         *string
	TrackLength       *int
	Pitboxes          *int
	DifficultyName    *string
	AbsAllowed        *int
	TcAllowed         *int
	StabilityAllowed  *int
	AutoclutchAllowed *int
	SessionName       *string
	BookingEnabled    *int
	BookingTime       *int
	PracticeEnabled   *int
	PracticeTime      *int
	QualifyEnabled    *int
	QualifyTime       *int
	RaceEnabled       *int
	RaceTime          *int
	ClassName         *string
	Entries           *int
	TimeName          *string
	Time              *string
	Graphics          *string
	TruncWeather      *int
	CspWeather        *int
	CacheTrackKey     *string
	CacheTrackConfig  *string
	CacheTrack        *string `form:"track" json:"track"`
	DifficultyId      *int    `form:"difficulty" json:"difficulty,string"`
	SessionId         *int    `form:"session" json:"session,string"`
	ClassId           *int    `form:"class" json:"class,string"`
	TimeId            *int    `form:"time" json:"time,string"`
}

type UserEventList struct {
	Id              *int    `json:"id"`
	EventCategoryId *int    `json:"event_category_id"`
	Name            *string `json:"name"`
	TrackName       *string `json:"track_name"`
}

type UserEventCategory struct {
	Id     *int        `json:"id"`
	Name   *string     `form:"name" json:"name"`
	Events []UserEvent `json:"events"`
}

type UserDifficulty struct {
	Id                      *int    `form:"id" json:"id"`
	Name                    *string `form:"name" json:"name"`
	AbsAllowed              *int    `form:"abs_allowed" json:"abs_allowed"`
	TcAllowed               *int    `form:"tc_allowed" json:"tc_allowed"`
	StabilityAllowed        *int    `form:"stability_allowed" json:"stability_allowed"`
	AutoclutchAllowed       *int    `form:"autoclutch_allowed" json:"autoclutch_allowed"`
	TyreBlanketsAllowed     *int    `form:"tyre_blankets_allowed" json:"tyre_blankets_allowed"`
	ForceVirtualMirror      *int    `form:"force_virtual_mirror" json:"force_virtual_mirror"`
	FuelRate                *int    `form:"fuel_rate" json:"fuel_rate"`
	DamageMultiplier        *int    `form:"damage_multiplier" json:"damage_multiplier"`
	TyreWearRate            *int    `form:"tyre_wear_rate" json:"tyre_wear_rate"`
	AllowedTyresOut         *int    `form:"allowed_tyres_out" json:"allowed_tyres_out"`
	MaxBallastKg            *int    `form:"max_ballast_kg" json:"max_ballast_kg"`
	StartRule               *int    `form:"start_rule" json:"start_rule"`
	RaceGasPenalityDisabled *int    `form:"race_gas_penality_disabled" json:"race_gas_penality_disabled"`
	DynamicTrack            *int    `form:"dynamic_track" json:"dynamic_track"`
	DynamicTrackPreset      *int    `form:"dynamic_track_preset" json:"dynamic_track_preset"`
	SessionStart            *int    `form:"session_start" json:"session_start"`
	Randomness              *int    `form:"randomness" json:"randomness"`
	SessionTransfer         *int    `form:"session_transfer" json:"session_transfer"`
	LapGain                 *int    `form:"lap_gain" json:"lap_gain"`
	KickQuorum              *int    `form:"kick_quorum" json:"kick_quorum"`
	VoteDuration            *int    `form:"vote_duration" json:"vote_duration"`
	VotingQuorum            *int    `form:"voting_quorum" json:"voting_quorum"`
	BlacklistMode           *int    `form:"blacklist_mode" json:"blacklist_mode"`
	MaxContactsPerKm        *int    `form:"max_contacts_per_km" json:"max_contacts_per_km"`
}

type UserSession struct {
	Id                    *int    `json:"id"`
	Name                  *string `form:"name" json:"name"`
	BookingEnabled        *int    `form:"booking_enabled" json:"booking_enabled"`
	BookingTime           *int    `form:"booking_time" json:"booking_time"`
	PracticeEnabled       *int    `form:"practice_enabled" json:"practice_enabled"`
	PracticeTime          *int    `form:"practice_time" json:"practice_time"`
	PracticeIsOpen        *int    `form:"practice_is_open" json:"practice_is_open"`
	QualifyEnabled        *int    `form:"qualify_enabled" json:"qualify_enabled"`
	QualifyTime           *int    `form:"qualify_time" json:"qualify_time"`
	QualifyIsOpen         *int    `form:"qualify_is_open" json:"qualify_is_open"`
	QualifyMaxWaitPerc    *int    `form:"qualify_max_wait_perc" json:"qualify_max_wait_perc"`
	RaceEnabled           *int    `form:"race_enabled" json:"race_enabled"`
	RaceTime              *int    `form:"race_time" json:"race_time"`
	RaceExtraLap          *int    `form:"race_extra_lap" json:"race_extra_lap"`
	RaceOverTime          *int    `form:"race_over_time" json:"race_over_time"`
	RaceWaitTime          *int    `form:"race_wait_time" json:"race_wait_time"`
	RaceIsOpen            *int    `form:"race_is_open" json:"race_is_open"`
	ReversedGridPositions *int    `form:"reversed_grid_positions" json:"reversed_grid_positions"`
	RacePitWindowStart    *int    `form:"race_pit_window_start" json:"race_pit_window_start"`
	RacePitWindowEnd      *int    `form:"race_pit_window_end" json:"race_pit_window_end"`
}

type UserTime struct {
	Id             *int              `json:"id"`
	Name           *string           `form:"name" json:"name"`
	Time           *string           `form:"time" json:"time"`
	TimeOfDayMulti *int              `form:"time_of_day_multi" json:"time_of_day_multi"`
	CspEnabled     *int              `form:"csp_enabled" json:"csp_enabled"`
	Weathers       []UserTimeWeather `json:"weathers"`
}

type UserTimeWeather struct {
	Id                     *int
	Name                   *string `json:"name"`
	UserTimeId             *int
	Graphics               *string `json:"graphics"`
	BaseTemperatureAmbient *int    `json:"base_temperature_ambient,string"`
	BaseTemperatureRoad    *int    `json:"base_temperature_road,string"`
	VariationAmbient       *int    `json:"variation_ambient,string"`
	VariationRoad          *int    `json:"variation_road,string"`
	WindBaseSpeedMin       *int    `json:"wind_base_speed_min,string"`
	WindBaseSpeedMax       *int    `json:"wind_base_speed_max,string"`
	WindBaseDirection      *int    `json:"wind_base_direction,string"`
	WindVariationDirection *int    `json:"wind_variation_direction,string"`
	CspTime                *string `json:"csp_time"`
	CspTimeOfDayMulti      *int    `json:"csp_time_of_day_multi,string"`
	CspDate                *string `json:"csp_date"`
}

type UserClass struct {
	Id      *int             `json:"id"`
	Name    *string          `form:"name" json:"name"`
	Entries []UserClassEntry `json:"entries"`
}

type UserClassEntry struct {
	Id          *int
	UserClassId *int    `json:"user_class_id"`
	CacheCarKey *string `json:"cache_car_key"`
	SkinKey     *string `json:"skin_key"`
	Ballast     *int    `json:"ballast"`
	Count       *int    `json:"count"`
}

type DropDownList struct {
	Id   *int    `json:"id"`
	Name *string `json:"name"`
}

type CacheCar struct {
	Id          *int
	Key         *string   `json:"key"`
	Name        *string   `json:"name"`
	Brand       *string   `json:"brand"`
	Desc        *string   `json:"description"`
	Tags        *[]string `json:"tags"`
	Class       *string   `json:"class"`
	ContentPath *string   `json:"content_path,omitempty"`
	ModifiedAt  *int64    `json:"modified_at,omitempty"`
	Specs struct {
		Bhp          string `json:"bhp"`
		Torque       string `json:"torque"`
		Weight       string `json:"weight"`
		Topspeed     string `json:"topspeed"`
		Acceleration string `json:"acceleration"`
		Pwratio      string `json:"pwratio"`
		Range        int    `json:"range"`
	} `json:"specs"`
	Torque [][]any `json:"torqueCurve"`
	Power  [][]any `json:"powerCurve"`
	Skins  []struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"skins"`
}

type CacheTrack struct {
	Id          *int
	Key         *string   `json:"key"`
	Config      *string   `json:"config"`
	Name        *string   `json:"name"`
	Desc        *string   `json:"desc"`
	Tags        *[]string `json:"tags"`
	Country     *string   `json:"country"`
	City        *string   `json:"city"`
	Length      *int      `json:"length"`
	Width       *string   `json:"width"`
	Pitboxes    *int      `json:"pitboxes,string"`
	Run         *string   `json:"run"`
	ContentPath *string   `json:"content_path,omitempty"`
	ModifiedAt  *int64    `json:"modified_at,omitempty"`
}

type CacheWeather struct {
	Id   *int
	Key  *string `json:"key"`
	Name *string `json:"name"`
}
