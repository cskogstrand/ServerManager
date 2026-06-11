package main

import (
	"bytes"
	"context"
	"flag"
	"html/template"
	"io"
	"log"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
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

func ConfigCompletedMiddlware() gin.HandlerFunc {
	return func(c *gin.Context) {
		configfilled, err := Dba.selectConfigFilled()
		if err != nil {
			log.Print("Database error: ", err)
			configfilled = false
		}
		if !configfilled && c.Request.URL.Path != "/config" && c.Request.URL.Path != "/content" {
			if debug {
				if user, ok := currentUserFromRequest(c); ok && user == "demo" {
					c.Set("user", user)
					c.Next()
					return
				}
			}
			c.Redirect(http.StatusFound, "/config")
			return
		}

		AuthenticateMiddleware(c)
	}
}

func AuthenticateMiddleware(c *gin.Context) {
	user, ok := currentUserFromRequest(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}

	c.Set("user", user)
	c.Next()
}

func main() {
	// Enables logging the filename
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	showVersion := flag.Bool("v", false, "display version information")
	flag.BoolVar(&debug, "debug", false, "serve templates and static files from disk for local development")
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

	mw := io.MultiWriter(os.Stdout, logfile, &LogBuffer)
	log.SetOutput(mw)

	TempFolder = filepath.Join(ConfigFolder, "tmp")
	if _, err := os.Stat(TempFolder); os.IsNotExist(err) {
		err := os.Mkdir(TempFolder, os.ModePerm)
		if err != nil {
			log.Fatal("Cannot create temp folder: ", err)
		}
	}

	dbpath := filepath.Join(ConfigFolder, "smdata.db")
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

	router := gin.New()
	if debug {
		router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
			SkipPaths: []string{"/api/server/status", "/api/content/jobs/active", "/api/server/events"},
		}))
	}
	router.Use(gin.Recovery())

	funcMap := template.FuncMap{
		"derefStr": func(t *string) string {
			if t == nil {
				return ""
			}
			return *t
		},
		"derefInt": func(t *int) string {
			if t == nil {
				return ""
			}
			return strconv.Itoa(*t)
		},
		"derefInt64": func(t *int64) string {
			if t == nil {
				return ""
			}
			return strconv.FormatInt(*t, 10)
		},
		"toInt": func(t *int) int {
			if t == nil {
				return 0
			}
			return *t
		},
		"toJsBool": func(t *int) bool {
			if t == nil {
				return false
			}
			if *t > 0 {
				return true
			}
			return false
		},
		"toTime": func(tm int) string {
			tdiff := time.Duration(tm) * time.Second
			return tdiff.Round(time.Second).String()
		},
		"inc": func(i int) int {
			return i + 1
		},
	}

	t := template.New("")
	t.Funcs(funcMap)
	err = LoadTemplate(t, ".htm")
	if err != nil {
		log.Fatal("Failed to load static template", err)
	}
	router.SetHTMLTemplate(t)

	if debug {
		router.Static("/static", "../")
	} else {
		router.StaticFS("/static", Assets)
	}

	// Vue SPA (Phase 1+), served alongside the legacy UI until cutover
	router.GET("/app/*path", routeSpa)

	router.GET("/login", routeLogin)
	router.POST("/login", routeLogin)
	router.GET("/logout", routeLogout)

	app := router.Group("/")
	app.Use(ConfigCompletedMiddlware())
	{
		app.GET("/", routeIndex)

		app.GET("/config", routeConfig)
		app.POST("/config", routeConfig)

		app.GET("/about", routeAbout)
		app.POST("/about", routeAbout)

		app.GET("/content", routeContent)
		app.POST("/content", routeContent)

		app.GET("/difficulty", routeDifficulty)
		app.POST("/difficulty", routeDifficulty)
		app.GET("/difficulty/:id", routeDifficulty)
		app.POST("/difficulty/:id", routeDifficulty)
		app.GET("/difficulty/delete/:id", routeDeleteDifficulty)
		app.POST("/difficulty/delete/:id", routeDeleteDifficulty)

		app.GET("/class", routeClass)
		app.POST("/class", routeClass)
		app.GET("/class/:id", routeClass)
		app.POST("/class/:id", routeClass)
		app.GET("/class/delete/:id", routeDeleteClass)
		app.POST("/class/delete/:id", routeDeleteClass)

		app.GET("/session", routeSession)
		app.POST("/session", routeSession)
		app.GET("/session/:id", routeSession)
		app.POST("/session/:id", routeSession)
		app.GET("/session/delete/:id", routeDeleteSession)
		app.POST("/session/delete/:id", routeDeleteSession)

		app.GET("/time", routeTime)
		app.POST("/time", routeTime)
		app.GET("/time/:id", routeTime)
		app.POST("/time/:id", routeTime)
		app.GET("/time/delete/:id", routeDeleteTime)
		app.POST("/time/delete/:id", routeDeleteTime)

		app.GET("/event", routeEventCategory)
		app.POST("/event", routeEventCategory)
		app.GET("/event/:id", routeEventCategory)
		app.POST("/event/:id", routeEventCategory)
		app.GET("/event/delete/:id", routeDeleteEventCategory)
		app.POST("/event/delete/:id", routeDeleteEventCategory)

		app.GET("/queue", routeQueue)
		app.POST("/queue", routeQueue)
		app.GET("/queue/delete/:id", routeDeleteQueue)
		app.POST("/queue/delete/:id", routeDeleteQueue)

		app.GET("/user", routeUser)
		app.POST("/user", routeUser)

		app.GET("/admin", routeAdmin)

		app.GET("/server", routeServer)
		app.GET("/mobile", routeMobile)
		app.GET("/mobile/track", routeMobileTrack)
		app.GET("/mobile/cars", routeMobileCars)
		app.GET("/mobile/weather", routeMobileWeather)
	}

	api := router.Group("/api")
	api.Use(AuthenticateMiddleware)
	api.Use(CsrfMiddleware)
	{
		api.GET("/car/:key", apiCar)
		api.GET("/car/image/:car/:skin", apiCarImage)

		api.GET("/track/preview/:track/:config", apiTrackPreviewImage)
		api.GET("/track/preview/:track", apiTrackPreviewImage)
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

		api.GET("/categories", apiCategoryList)
		api.POST("/categories", apiCategoryCreate)
		api.GET("/category/:id", apiCategoryGet)
		api.PUT("/category/:id", apiCategoryUpdate)
		api.DELETE("/category/:id", apiCategoryDelete)

		api.GET("/events", apiEventList)
		api.POST("/events", apiEventCreate)
		api.GET("/event/:id", apiEventGet)
		api.PUT("/event/:id", apiEventUpdate)
		api.DELETE("/event/:id", apiEventDelete)

		api.GET("/config", apiConfigGet)
		api.PUT("/config", apiConfigUpdate)
		api.PUT("/config/content", apiConfigContentUpdate)

		api.GET("/user", apiUserGet)
		api.PUT("/user", apiUserUpdate)

		api.POST("/content/recache", apiRecacheContent)
		api.GET("/content/jobs/active", apiContentJobsActive)
		api.GET("/content/jobs/:id", apiContentJob)
		api.POST("/content/upload", apiContentUpload)

		api.POST("/validate/installpath", apiValidateInstallpath)

		api.POST("/server/start", apiServerStart)
		api.POST("/server/stop", apiServerStop)
		api.GET("/server/status", apiServerStatus)
		api.GET("/server/events", apiServerEventsSSE)
		api.POST("/server/current-event", apiServerUpdateCurrentEvent)
		api.GET("/server/logfile", apiServerLogfile)
		api.GET("/server/smdata", apiServerSmdata)
		api.GET("/server/smcontent", apiServerSmcontent)

		api.GET("/server/entry_list.ini", apiEntryList)
		api.GET("/server/server_cfg.ini", apiServerCfg)

		api.POST("/queue/moveup/:id", apiQueueMoveUp)
		api.POST("/queue/movedown/:id", apiQueueMoveDown)
		api.POST("/queue/skipevent", apiQueueSkipEvent)
		api.POST("/queue/clearcompleted", apiQueueClearCompleted)
		api.POST("/queue/event/:id", apiQueueAddEvent)
		api.POST("/queue/category/:id", apiQueueAddCategory)

		api.GET("/instances", apiInstances)
		api.POST("/instances", apiInstanceCreate)
		api.PUT("/instances/:id", apiInstanceUpdate)
		api.DELETE("/instances/:id", apiInstanceDelete)
	}

	router.NoRoute(route404)

	if !debug {
		OpenURL("http://localhost:3030")
	}

	updatePublicIp()

	if cfg.AutoStartServer != nil && *cfg.AutoStartServer > 0 {
		go func() {
			inst := Instances.Default()
			if inst == nil || inst.isRunning() {
				return
			}
			if inst.serverApplyTrack() {
				log.Print("Auto-starting server from queue on launch")
				inst.start()
			} else {
				log.Print("Auto-start enabled but no unfinished queue event was available")
			}
		}()
	}

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
