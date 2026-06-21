package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var Dba Dbaccess
var Instances *InstanceManager
var ConfigFolder string
var TempFolder string
var SecretKey []byte
var Zf ZipFile
var LogBuffer bytes.Buffer
var ContentJobs *ContentJobStore
var Version = "development"

// TODO: checksuming is failing when CSP is enabled
var debug bool = false

func currentUserFromRequest(c *gin.Context) (string, bool) {
	tokenString, err := c.Cookie("token")
	if err != nil {
		return "", false
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return SecretKey, nil
	})
	if err != nil || !token.Valid {
		return "", false
	}

	user, err := token.Claims.GetSubject()
	if err != nil || user == "" {
		return "", false
	}

	return user, true
}

func AuthenticateMiddleware(c *gin.Context) {
	user, ok := currentUserFromRequest(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{"code": "unauthorized", "message": "Not logged in"},
		})
		return
	}

	// Resolve the role fresh each request so a role change takes effect without
	// re-login; a missing user row means the account was deleted.
	usr, err := Dba.selectUser(user)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{"code": "unauthorized", "message": "Session no longer valid"},
		})
		return
	}
	role := roleAdmin
	if usr.Role != nil && *usr.Role != "" {
		role = *usr.Role
	}

	c.Set("user", user)
	c.Set("role", role)
	c.Next()
}

func main() {
	// Enables logging the filename
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	showVersion := flag.Bool("v", false, "display version information")
	flag.BoolVar(&debug, "debug", false, "serve embedded assets from disk for local development")
	flag.StringVar(&ConfigFolder, "p", "", "Configuration path")
	flag.Parse()

	if *showVersion {
		fmt.Printf("SM version: %s\n", Version)
		os.Exit(0)
	}

	if ConfigFolder == "" {
		ConfigFolder = os.Getenv("XDG_CONFIG_HOME")
		if runtime.GOOS == "windows" {
			ConfigFolder = os.Getenv("APPDATA")
		}
	}

	ConfigFolder = filepath.Join(ConfigFolder, "servermanager")
	if _, err := os.Stat(ConfigFolder); os.IsNotExist(err) {
		err := os.Mkdir(ConfigFolder, os.ModePerm)
		if err != nil {
			log.Fatal("Cannot create config folder: ", err)
		}
	}

	logpath := filepath.Join(ConfigFolder, "logfile.log")
	log.Print("Opening log file located at: ", logpath)
	logfile, err := os.OpenFile(logpath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("error opening file: ", err)
	}
	defer logfile.Close()

	mw := io.MultiWriter(os.Stdout, logfile, &LogBuffer, LogLines)
	log.SetOutput(mw)

	TempFolder = filepath.Join(ConfigFolder, "tmp")
	if _, err := os.Stat(TempFolder); os.IsNotExist(err) {
		err := os.Mkdir(TempFolder, os.ModePerm)
		if err != nil {
			log.Fatal("Cannot create temp folder: ", err)
		}
	}

	dbpath := filepath.Join(ConfigFolder, "smdata.db")
	// A restore staged via the maintenance page is swapped in here, before the
	// DB is opened, so we never replace a file held open by *sql.DB.
	applyStagedRestore(dbpath)
	log.Print("Opening database file located at: " + dbpath)
	Dba = open(dbpath)
	Dba.applySchema("/schema.sql")

	cfg, err := Dba.selectConfig()
	if err != nil {
		log.Fatal("Database error: ", err)
	}

	SecretKey = []byte(*cfg.SecretKey)

	gin.SetMode(gin.ReleaseMode)

	Zf = ZipFile{}
	ContentJobs = NewContentJobStore()

	Instances = NewInstanceManager()
	if err := Instances.LoadFromDb(); err != nil {
		log.Fatal("Could not load server instances: ", err)
	}

	// Drift-spike stream capture (no-op if ffmpeg is absent or no driver has a
	// capture URL configured).
	Captures = newCaptureManager()

	// Token gating the public drift telemetry WebSocket, baked into the script
	// served to clients.
	initDriftIngestToken()

	router := gin.New()
	if debug {
		router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
			SkipPaths: []string{"/api/server/status", "/api/content/jobs/active", "/api/server/events"},
		}))
	}
	router.Use(gin.Recovery())

	router.StaticFS("/static", StaticAssetsFS())

	// Pre-cutover bookmarks
	router.GET("/app/*path", routeLegacyApp)

	// Public mod downloads — joining players are not authenticated.
	router.GET("/dl/car/:key", apiDownloadCar)
	router.GET("/dl/track/:key", apiDownloadTrack)

	// Public drift telemetry — a joining player's CSP script fetches the
	// rendered HUD/feeder script (/sm/lua/driftscore) and streams slip data back
	// over the WebSocket (/telemetry/ingest). Authenticated by the per-process
	// ingest token baked into the served script, not the JWT cookie (game
	// clients have none).
	router.GET("/sm/lua/driftscore", apiDriftLuaScript)
	router.GET("/telemetry/ingest", apiTelemetryIngest)

	// Login is the only unauthenticated API endpoint
	router.POST("/api/login", apiLogin)

	api := router.Group("/api")
	api.Use(AuthenticateMiddleware)
	api.Use(CsrfMiddleware)
	api.Use(RoleMiddleware)
	{
		api.GET("/cars", apiCarsList)
		api.GET("/tracks", apiTracksList)
		api.GET("/weathers", apiWeathersList)

		api.GET("/car/:key", apiCar)
		api.GET("/car/image/:car/:skin", apiCarImage)

		api.GET("/track/preview/:track/:config", apiTrackPreviewImage)
		api.GET("/track/preview/:track", apiTrackPreviewImage)
		api.GET("/track/map/:track/:config", apiTrackMapImage)
		api.GET("/track/map/:track", apiTrackMapImage)
		api.GET("/track/mapmeta/:track/:config", apiTrackMapMeta)
		api.GET("/track/mapmeta/:track", apiTrackMapMeta)
		api.GET("/track/outline/:track/:config", apiTrackOutlineImage)
		api.GET("/track/outline/:track", apiTrackOutlineImage)
		api.GET("/weather/preview/:weather", apiWeatherPreviewImage)

		api.GET("/difficulties", apiDifficultyList)
		api.POST("/difficulties", apiDifficultyCreate)
		api.GET("/difficulty/:id", apiDifficulty)
		api.PUT("/difficulty/:id", apiDifficultyUpdate)
		api.DELETE("/difficulty/:id", apiDifficultyDelete)

		api.GET("/sessions", apiSessionList)
		api.POST("/sessions", apiSessionCreate)
		api.GET("/session/:id", apiSession)
		api.PUT("/session/:id", apiSessionUpdate)
		api.DELETE("/session/:id", apiSessionDelete)

		api.GET("/times", apiTimeList)
		api.POST("/times", apiTimeCreate)
		api.GET("/time/:id", apiTime)
		api.PUT("/time/:id", apiTimeUpdate)
		api.DELETE("/time/:id", apiTimeDelete)

		api.GET("/classes", apiClassList)
		api.POST("/classes", apiClassCreate)
		api.GET("/class/:id", apiClass)
		api.PUT("/class/:id", apiClassUpdate)
		api.DELETE("/class/:id", apiClassDelete)

		api.GET("/presets/usage", apiPresetUsage)

		api.GET("/categories", apiCategoryList)
		api.POST("/categories", apiCategoryCreate)
		api.GET("/category/:id", apiCategoryGet)
		api.PUT("/category/:id", apiCategoryUpdate)
		api.PATCH("/category/:id", apiCategoryRename)
		api.POST("/category/:id/duplicate", apiCategoryDuplicate)
		api.DELETE("/category/:id", apiCategoryDelete)

		api.GET("/events", apiEventList)
		api.POST("/events", apiEventCreate)
		api.GET("/event/:id", apiEventGet)
		api.PUT("/event/:id", apiEventUpdate)
		api.DELETE("/event/:id", apiEventDelete)

		api.GET("/config", apiConfigGet)
		api.PUT("/config", apiConfigUpdate)
		api.PUT("/config/content", apiConfigContentUpdate)
		api.GET("/server/engine", apiServerEngine)
		api.POST("/server/assettoserver/install", apiAssettoServerInstall)

		api.GET("/user", apiUserGet)
		api.PUT("/user", apiUserUpdate)
		api.POST("/logout", apiLogout)
		api.GET("/about", apiAbout)

		api.GET("/users", apiUsersList)
		api.POST("/users", apiUserCreate)
		api.PUT("/users/:name/role", apiUserSetRole)
		api.PUT("/users/:name/password", apiUserResetPassword)
		api.DELETE("/users/:name", apiUserDelete)

		api.POST("/content/recache", apiRecacheContent)
		api.POST("/content/compress", apiCompressImages)
		api.GET("/content/images/count", apiContentImageCount)
		api.GET("/content/jobs/active", apiContentJobsActive)
		api.GET("/content/jobs/:id", apiContentJob)
		api.POST("/content/upload", apiContentUpload)
		api.DELETE("/track/:key", apiTrackDelete)
		api.DELETE("/car/:key", apiCarDelete)
		api.DELETE("/weather/:key", apiWeatherDelete)

		api.POST("/validate/installpath", apiValidateInstallpath)

		api.POST("/server/start", apiServerStart)
		api.POST("/server/stop", apiServerStop)
		api.GET("/server/status", apiServerStatus)
		api.GET("/server/readiness", apiServerReadiness)
		api.GET("/setup/summary", apiSetupSummary)
		api.GET("/server/events", apiServerEventsSSE)
		api.POST("/server/current-event", apiServerUpdateCurrentEvent)
		api.POST("/server/broadcast", apiRaceBroadcast)
		api.POST("/server/next-session", apiRaceNextSession)
		api.POST("/server/restart-session", apiRaceRestartSession)
		api.POST("/server/admin-command", apiRaceAdminCommand)
		api.POST("/server/kick", apiRaceKick)
		api.GET("/server/logfile", apiServerLogfile)
		api.GET("/server/smdata", apiServerSmdata)
		api.GET("/server/smcontent", apiServerSmcontent)
		api.POST("/maintenance/restore", apiMaintenanceRestore)

		api.GET("/server/entry_list.ini", apiEntryList)
		api.GET("/server/server_cfg.ini", apiServerCfg)
		api.POST("/server/render-preview", apiRenderPreview)

		api.POST("/queue/moveup/:id", apiQueueMoveUp)
		api.POST("/queue/movedown/:id", apiQueueMoveDown)
		api.PUT("/queue/order", apiQueueReorder)
		api.POST("/queue/skipevent", apiQueueSkipEvent)
		api.POST("/queue/clearcompleted", apiQueueClearCompleted)
		api.POST("/queue/event/:id", apiQueueAddEvent)
		api.POST("/queue/category/:id", apiQueueAddCategory)
		api.GET("/queue", apiQueueList)
		api.DELETE("/queue/:id", apiQueueDelete)

		api.GET("/instances", apiInstances)
		api.POST("/instances", apiInstanceCreate)
		api.GET("/instances/:id/stream/status", apiInstanceStreamStatus)
		api.GET("/instances/:id/driver-streams/status", apiInstanceDriverStreamStatuses)
		api.PUT("/instances/:id", apiInstanceUpdate)
		api.PUT("/instances/:id/runmode", apiInstanceRunMode)
		api.PUT("/instances/:id/schedule", apiInstanceSchedule)
		api.DELETE("/instances/:id", apiInstanceDelete)
		api.GET("/driver-streams", apiDriverStreamsList)
		api.POST("/driver-streams", apiDriverStreamCreate)
		api.PUT("/driver-streams/:id", apiDriverStreamUpdate)
		api.DELETE("/driver-streams/:id", apiDriverStreamDelete)

		api.GET("/streams/debug", apiStreamsDebug)
		api.POST("/streams/debug/probe", apiStreamsProbe)
		api.GET("/streams/capture-status", apiCaptureStatus)

		api.GET("/drivers", apiDriversList)
		api.GET("/scores", apiScoresList)
		api.GET("/drivers/:guid", apiDriverGet)
		api.GET("/drivers/:guid/avatar", apiDriverAvatar)
		api.POST("/drivers/:guid/avatar", apiDriverAvatarUpload)
		api.POST("/drivers/:guid/record", apiDriverRecord)
		api.POST("/drivers/:guid/record/stop", apiDriverRecordStop)
		api.POST("/drivers/:guid/snapshot", apiDriverSnapshot)
		api.GET("/drivers/:guid/media/:file", apiDriverMedia)
		api.DELETE("/drivers/:guid/media/:file", apiDriverMediaDelete)

		// Extra drivers: shared-account roster, per-row leaderboard reassignment,
		// and tagging a live car so its results log under the right person.
		api.GET("/extra-drivers", apiExtraDriversList)
		api.POST("/extra-drivers", apiExtraDriverCreate)
		api.PUT("/extra-drivers/:id", apiExtraDriverUpdate)
		api.DELETE("/extra-drivers/:id", apiExtraDriverDelete)
		api.POST("/scores/:id/assign", apiScoreAssign)
		api.POST("/server/assign-driver", apiLiveDriverAssign)
	}

	// Everything that is not /api or /static is the SPA
	router.NoRoute(routeSpa)

	if !debug {
		OpenURL("http://localhost:3030")
	}

	updatePublicIp()
	startScheduler()

	go func() {
		// Legacy global toggle keeps starting the default instance; the
		// per-instance start_on_boot flag covers everything else.
		globalAutoStart := cfg.AutoStartServer != nil && *cfg.AutoStartServer > 0
		for _, inst := range Instances.All() {
			onBoot := inst.Conf.StartOnBoot != nil && *inst.Conf.StartOnBoot > 0
			if !onBoot && !(globalAutoStart && inst == Instances.Default()) {
				continue
			}
			if inst.isRunning() {
				continue
			}
			if ok, err := inst.serverApplyTrack(); ok {
				log.Printf("Auto-starting instance %q from queue on launch", inst.Name())
				inst.start()
			} else if err != nil {
				log.Printf("Auto-start of instance %q failed: %v", inst.Name(), err)
			} else {
				log.Printf("Auto-start enabled for instance %q but no unfinished queue event was available", inst.Name())
			}
		}
	}()

	main := &http.Server{
		Addr:    ":3030",
		Handler: router.Handler(),
	}

	go func() {
		if err := main.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("WebUI could not start up on port 3030: ", err)
		} else {
			log.Print("WebUI up and running on http://localhost:3030")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Print("Shutting down WebUI...\n\n")

	Dba.db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := main.Shutdown(ctx); err != nil {
		log.Print("Shutdown error: ", err)
	}
	select {
	case <-ctx.Done():
	}

}
