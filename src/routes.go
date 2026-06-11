package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func sref(v string) *string { return &v }
func iref(v int) *int       { return &v }

func isDemoRequest(c *gin.Context) bool {
	if !debug {
		return false
	}
	user, ok := currentUserFromRequest(c)
	return ok && user == "demo"
}

func routeConfigFilled(c *gin.Context) (bool, error) {
	if isDemoRequest(c) {
		return true, nil
	}
	return Dba.selectConfigFilled()
}

func demoTracks() []CacheTrack {
	baseNames := []string{
		"Neon Docks", "Boreal Ring", "Sungrid Valley", "Blackforest Pass", "Hinterland Yard",
		"Redline Estate", "Coastline Loop", "Silver Quarry", "Northwind Circuit", "Delta Freight Park",
	}
	configs := []string{"layout_a", "gp", "club", "reverse", "drift"}
	countries := []string{"Japan", "Norway", "USA", "Germany", "Sweden", "Italy", "Spain", "Canada", "France", "UK"}
	cities := []string{"Yokohama", "Tromso", "Palm Springs", "Freiburg", "Malmo", "Modena", "Bilbao", "Calgary", "Lyon", "Leeds"}
	tagsPool := [][]string{
		{"drift", "night", "harbor"},
		{"snow", "technical", "touge"},
		{"desert", "club", "time attack"},
		{"mountain", "reverse", "road"},
		{"industrial", "practice", "open"},
	}

	tracks := make([]CacheTrack, 0, 50)
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("%s %02d", baseNames[i%len(baseNames)], i+1)
		key := fmt.Sprintf("demo_track_%02d", i+1)
		config := configs[i%len(configs)]
		country := countries[i%len(countries)]
		city := cities[i%len(cities)]
		length := 1800 + (i * 137)
		pitboxes := 14 + (i % 20)
		tags := tagsPool[i%len(tagsPool)]

		tracks = append(tracks, CacheTrack{
			Key:      sref(key),
			Config:   sref(config),
			Name:     sref(name),
			Country:  sref(country),
			City:     sref(city),
			Length:   iref(length),
			Pitboxes: iref(pitboxes),
			Tags:     &tags,
		})
	}
	return tracks
}

func demoCars() []CacheCar {
	baseNames := []string{
		"Silvia S13 Demo", "E36 Street Demo", "MX-5 ND Cup Demo", "GT86 Slide Demo", "997 GT3 Touring Demo",
		"Chaser JZX100 Demo", "Mustang Foxbody Demo", "M3 E92 Sprint Demo", "RX-7 FD Attack Demo", "Supra A90 Demo",
	}
	brands := []string{"Nissan", "BMW", "Mazda", "Toyota", "Porsche", "Ford", "Lexus", "Chevrolet", "Subaru", "Mercedes"}
	classes := []string{"Drift", "Street", "Cup", "Track", "Touge"}
	tagsPool := [][]string{
		{"drift", "rwd", "turbo"},
		{"street", "balanced", "rwd"},
		{"cup", "lightweight", "beginner"},
		{"track", "grip", "fast"},
		{"touge", "agile", "night"},
	}

	cars := make([]CacheCar, 0, 50)
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("%s %02d", baseNames[i%len(baseNames)], i+1)
		key := fmt.Sprintf("demo_car_%02d", i+1)
		brand := brands[i%len(brands)]
		class := classes[i%len(classes)]
		tags := tagsPool[i%len(tagsPool)]
		skins := []struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		}{
			{Key: fmt.Sprintf("skin_%02d_a", i+1), Name: fmt.Sprintf("Factory %02d", i+1)},
			{Key: fmt.Sprintf("skin_%02d_b", i+1), Name: fmt.Sprintf("Livery %02d", i+1)},
		}

		cars = append(cars, CacheCar{
			Key:   sref(key),
			Name:  sref(name),
			Brand: sref(brand),
			Class: sref(class),
			Tags:  &tags,
			Skins: skins,
		})
	}
	return cars
}

func demoWeathers() []CacheWeather {
	baseNames := []string{
		"Clear Cold Morning", "Golden Hour", "Soft Overcast", "Neon Rain", "Blue Dusk",
		"Late Summer Haze", "Dense Fog", "Coastal Storm", "Dry Heat", "Moonlit Clear",
	}
	weathers := make([]CacheWeather, 0, 50)
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("%s %02d", baseNames[i%len(baseNames)], i+1)
		key := fmt.Sprintf("demo_weather_%02d", i+1)
		weathers = append(weathers, CacheWeather{
			Key:  sref(key),
			Name: sref(name),
		})
	}
	return weathers
}

func withDemoTracks(c *gin.Context, tracks []CacheTrack) []CacheTrack {
	if isDemoRequest(c) {
		return append(tracks, demoTracks()...)
	}
	return tracks
}

func withDemoCars(c *gin.Context, cars []CacheCar) []CacheCar {
	if isDemoRequest(c) {
		return append(cars, demoCars()...)
	}
	return cars
}

func withDemoWeathers(c *gin.Context, weathers []CacheWeather) []CacheWeather {
	if isDemoRequest(c) {
		return append(weathers, demoWeathers()...)
	}
	return weathers
}

func routeAbout(c *gin.Context) {
	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}


	c.HTML(http.StatusOK, "/htm/about.htm", gin.H{
		"page":          "about",
		"config_filled": cfgFilled,
		"cfgLoc":        ConfigFolder,
		"tmpLoc":        TempFolder,
		"status":        defaultStatus(),
	})
}

func routeConfig(c *gin.Context) {
	var form UserConfig
	if c.Request.Method == "POST" && c.ShouldBind(&form) == nil {
		_, err := Dba.updateConfig(form)
		if err != nil {
			routeDbError(c, err)
			return
		}
	}


	form, err := Dba.selectConfig()
	if err != nil {
		routeDbError(c, err)
		return
	}

	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}

	c.HTML(http.StatusOK, "/htm/config.htm", gin.H{
		"page":          "config",
		"form":          form,
		"config_filled": cfgFilled,
		"status":        defaultStatus(),
	})
}

func routeContent(c *gin.Context) {
	var form UserConfig
	if c.Request.Method == "POST" && c.ShouldBind(&form) == nil {
		_, err := Dba.updateContent(form)
		if err != nil {
			routeDbError(c, err)
			return
		}
	}

	form, err := Dba.selectConfig()
	if err != nil {
		routeDbError(c, err)
		return
	}

	trackData, err := Dba.selectCacheTracks()
	if err != nil {
		routeDbError(c, err)
		return
	}

	carData, err := Dba.selectCacheCars()
	if err != nil {
		routeDbError(c, err)
		return
	}

	weatherData, err := Dba.selectCacheWeathers()
	if err != nil {
		routeDbError(c, err)
		return
	}
	trackData = withDemoTracks(c, trackData)
	carData = withDemoCars(c, carData)
	weatherData = withDemoWeathers(c, weatherData)

	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}

	c.HTML(http.StatusOK, "/htm/content.htm", gin.H{
		"page":          "content",
		"form":          form,
		"track_data":    trackData,
		"car_data":      carData,
		"weather_data":  weatherData,
		"config_filled": cfgFilled,
		"status":        defaultStatus(),
	})
}

func routeIndex(c *gin.Context) {
	c.Redirect(http.StatusFound, "/server")
}

func routeServer(c *gin.Context) {
	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}
	trackData, err := Dba.selectCacheTracks()
	if err != nil {
		routeDbError(c, err)
		return
	}
	carData, err := Dba.selectCacheCars()
	if err != nil {
		routeDbError(c, err)
		return
	}
	weatherData, err := Dba.selectCacheWeathers()
	if err != nil {
		routeDbError(c, err)
		return
	}
	trackData = withDemoTracks(c, trackData)
	carData = withDemoCars(c, carData)
	weatherData = withDemoWeathers(c, weatherData)
	classes, err := Dba.selectClassList(true)
	if err != nil {
		routeDbError(c, err)
		return
	}
	times, err := Dba.selectTimeList(true)
	if err != nil {
		routeDbError(c, err)
		return
	}
	c.HTML(http.StatusOK, "/htm/server.htm", gin.H{
		"page":          "server",
		"config_filled": cfgFilled,
		"track_data":    trackData,
		"car_data":      carData,
		"weather_data":  weatherData,
		"classes":       classes,
		"times":         times,
		"tmpLoc":        TempFolder,
		"status":        defaultStatus(),
	})
}

func routeMobile(c *gin.Context) {
	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}
	weatherData, err := Dba.selectCacheWeathers()
	if err != nil {
		routeDbError(c, err)
		return
	}
	weatherData = withDemoWeathers(c, weatherData)
	c.HTML(http.StatusOK, "/htm/mobile.htm", gin.H{
		"page":          "mobile",
		"title":         " · Mobile",
		"config_filled": cfgFilled,
		"tmpLoc":        TempFolder,
		"weather_data":  weatherData,
		"status":        defaultStatus(),
	})
}

func routeMobileTrack(c *gin.Context) {
	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}
	trackData, err := Dba.selectCacheTracks()
	if err != nil {
		routeDbError(c, err)
		return
	}
	trackData = withDemoTracks(c, trackData)
	c.HTML(http.StatusOK, "/htm/mobile_track.htm", gin.H{
		"page":          "mobile",
		"config_filled": cfgFilled,
		"track_data":    trackData,
		"status":        defaultStatus(),
	})
}

func routeMobileCars(c *gin.Context) {
	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}
	carData, err := Dba.selectCacheCars()
	if err != nil {
		routeDbError(c, err)
		return
	}
	carData = withDemoCars(c, carData)
	c.HTML(http.StatusOK, "/htm/mobile_cars.htm", gin.H{
		"page":          "mobile",
		"config_filled": cfgFilled,
		"car_data":      carData,
		"status":        defaultStatus(),
	})
}

func routeMobileWeather(c *gin.Context) {
	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}
	weatherData, err := Dba.selectCacheWeathers()
	if err != nil {
		routeDbError(c, err)
		return
	}
	weatherData = withDemoWeathers(c, weatherData)
	c.HTML(http.StatusOK, "/htm/mobile_weather.htm", gin.H{
		"page":          "mobile",
		"config_filled": cfgFilled,
		"weather_data":  weatherData,
		"status":        defaultStatus(),
	})
}

func routeQueue(c *gin.Context) {
	if c.Request.Method == "POST" {
		instanceId, _ := strconv.Atoi(c.PostForm("instance"))
		if instanceId <= 0 {
			if inst := Instances.Default(); inst != nil {
				instanceId = inst.Id()
			}
		}

		if c.PostForm("event") != "" {
			// insert single event from category
			id, err := strconv.Atoi(c.PostForm("event"))
			if id > 0 && err == nil {
				_, err := Dba.insertServerEvent(id, instanceId)
				if err != nil {
					routeDbError(c, err)
					return
				}
			}
		} else {
			// insert all events from category
			id, err := strconv.Atoi(c.PostForm("category"))
			if id > 0 && err == nil {
				_, err := Dba.insertServerEventCategory(id, instanceId)
				if err != nil {
					routeDbError(c, err)
					return
				}
			}
		}
	}

	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}

	eventCat, err := Dba.selectEventCategoryList(false)
	if err != nil {
		routeDbError(c, err)
		return
	}

	eventList, err := Dba.selectEventList()
	if err != nil {
		routeDbError(c, err)
		return
	}

	serverEvents, err := Dba.selectServerEvents(false, 0)
	if err != nil {
		routeDbError(c, err)
		return
	}

	c.HTML(http.StatusOK, "/htm/queue.htm", gin.H{
		"page":          "queue",
		"config_filled": cfgFilled,
		"event_cat":     eventCat,
		"event_list":    eventList,
		"server_events": serverEvents,
		"instances":     Instances.All(),
		"tmpLoc":        TempFolder,
		"status":        defaultStatus(),
	})
}

func routeDeleteQueue(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := Dba.deleteServerEvent(id)

	if err != nil {
		routeFkError(c, err)
	} else {
		c.Redirect(http.StatusFound, "/queue")
	}
}

func routeDifficulty(c *gin.Context) {
	form := UserDifficulty{}
	id, err := strconv.Atoi(c.Param("id"))
	if id > 0 && err == nil {
		form, err = Dba.selectDifficulty(id)

		if err != nil {
			routeDbError(c, err)
			return
		}

		if form.Id == nil {
			route404(c)
			return
		}
	}
	if c.Request.Method == "POST" {
		if c.PostForm("difficulty_name") != "" {
			id, err := Dba.insertDifficulty(c.PostForm("difficulty_name"))
			if err != nil {
				routeDbError(c, err)
				return
			}
			c.Redirect(http.StatusFound, fmt.Sprint("/difficulty/", id))
			return
		} else if c.ShouldBind(&form) == nil {
			_, err := Dba.updateDifficulty(form)
			if err != nil {
				routeDbError(c, err)
				return
			}
		}
	}

	list, err := Dba.selectDifficultyList(false)
	if err != nil {
		routeDbError(c, err)
		return
	}

	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}

	c.HTML(http.StatusOK, "/htm/difficulty.htm", gin.H{
		"page":          "difficulty",
		"list":          list,
		"form":          form,
		"config_filled": cfgFilled,
		"status":        defaultStatus(),
	})
}

func routeDeleteDifficulty(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := Dba.deleteDifficulty(id)

	if err != nil {
		routeFkError(c, err)
	} else {
		c.Redirect(http.StatusFound, "/difficulty")
	}
}

func routeClass(c *gin.Context) {
	form := UserClass{}
	id, err := strconv.Atoi(c.Param("id"))
	if id > 0 && err == nil {
		form, err = Dba.selectClassEntries(id)
		if err != nil {
			routeDbError(c, err)
			return
		}

		if form.Id == nil {
			route404(c)
			return
		}
	}
	if c.Request.Method == "POST" {
		if c.PostForm("class_name") != "" {
			id, err := Dba.insertClass(c.PostForm("class_name"))
			if err != nil {
				routeDbError(c, err)
				return
			}
			c.Redirect(http.StatusFound, fmt.Sprint("/class/", id))
			return
		} else if c.ShouldBind(&form) == nil {
			json.Unmarshal([]byte(c.PostForm("entries")), &form.Entries)
			_, err := Dba.updateClass(form)
			if err != nil {
				routeDbError(c, err)
				return
			}
		}
	}

	list, err := Dba.selectClassList(false)
	if err != nil {
		routeDbError(c, err)
		return
	}

	carData, err := Dba.selectCacheCars()
	if err != nil {
		routeDbError(c, err)
		return
	}
	carData = withDemoCars(c, carData)

	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}

	c.HTML(http.StatusOK, "/htm/class.htm", gin.H{
		"page":          "class",
		"list":          list,
		"car_data":      carData,
		"form":          form,
		"config_filled": cfgFilled,
		"status":        defaultStatus(),
	})
}

func routeDeleteClass(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := Dba.deleteClass(id)

	if err != nil {
		routeFkError(c, err)
	} else {
		c.Redirect(http.StatusFound, "/class")
	}
}

func routeSession(c *gin.Context) {
	form := UserSession{}
	id, err := strconv.Atoi(c.Param("id"))
	if id > 0 && err == nil {
		form, err = Dba.selectSession(id)
		if err != nil {
			routeDbError(c, err)
			return
		}

		if form.Id == nil {
			route404(c)
			return
		}
	}
	if c.Request.Method == "POST" {
		if c.PostForm("session_name") != "" {
			id, err := Dba.insertSession(c.PostForm("session_name"))
			if err != nil {
				routeDbError(c, err)
				return
			}
			c.Redirect(http.StatusFound, fmt.Sprint("/session/", id))
			return
		} else if c.ShouldBind(&form) == nil {
			_, err := Dba.updateSession(form)
			if err != nil {
				routeDbError(c, err)
				return
			}
		}
	}

	list, err := Dba.selectSessionList(false)
	if err != nil {
		routeDbError(c, err)
		return
	}

	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}

	c.HTML(http.StatusOK, "/htm/session.htm", gin.H{
		"page":          "session",
		"list":          list,
		"form":          form,
		"config_filled": cfgFilled,
		"status":        defaultStatus(),
	})
}

func routeDeleteSession(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := Dba.deleteSession(id)

	if err != nil {
		routeFkError(c, err)
	} else {
		c.Redirect(http.StatusFound, "/session")
	}
}

func routeTime(c *gin.Context) {
	form := UserTime{}
	id, err := strconv.Atoi(c.Param("id"))
	if id > 0 && err == nil {
		form, err = Dba.selectTimeWeather(id)
		if err != nil {
			routeDbError(c, err)
			return
		}

		if form.Id == nil {
			route404(c)
			return
		}
	}
	if c.Request.Method == "POST" {
		if c.PostForm("time_name") != "" {
			id, err := Dba.insertTime(c.PostForm("time_name"))
			if err != nil {
				routeDbError(c, err)
				return
			}
			c.Redirect(http.StatusFound, fmt.Sprint("/time/", id))
			return
		} else if c.ShouldBind(&form) == nil {
			json.Unmarshal([]byte(c.PostForm("weather")), &form.Weathers)
			_, err := Dba.updateTime(form)
			if err != nil {
				routeDbError(c, err)
				return
			}
		}
	}

	if len(form.Weathers) == 0 {
		form.Weathers = append(form.Weathers, UserTimeWeather{})
	}

	list, err := Dba.selectTimeList(false)
	if err != nil {
		routeDbError(c, err)
		return
	}

	weatherList, err := Dba.selectCacheWeathers()
	if err != nil {
		routeDbError(c, err)
		return
	}
	weatherList = withDemoWeathers(c, weatherList)

	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}

	c.HTML(http.StatusOK, "/htm/time.htm", gin.H{
		"page":          "time",
		"list":          list,
		"weatherlist":   weatherList,
		"form":          form,
		"config_filled": cfgFilled,
		"status":        defaultStatus(),
	})
}

func routeDeleteTime(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := Dba.deleteTime(id)

	if err != nil {
		routeFkError(c, err)
	} else {
		c.Redirect(http.StatusFound, "/time")
	}
}

func routeEventCategory(c *gin.Context) {
	form := UserEventCategory{}

	id, err := strconv.Atoi(c.Param("id"))
	if id > 0 && err == nil {
		form, err = Dba.selectCategoryEvents(id)
		if err != nil {
			routeDbError(c, err)
			return
		}

		if form.Id == nil {
			route404(c)
			return
		}
	}
	if c.Request.Method == "POST" {
		if c.PostForm("category_name") != "" {
			id, err := Dba.insertEventCategory(c.PostForm("category_name"))
			if err != nil {
				routeDbError(c, err)
				return
			}
			c.Redirect(http.StatusFound, fmt.Sprint("/event/", id))
			return
		} else if c.ShouldBind(&form) == nil {
			json.Unmarshal([]byte(c.PostForm("events")), &form.Events)
			_, err := Dba.updateEventCategory(form)
			if err != nil {
				routeDbError(c, err)
				return
			}
		}
	}

	if len(form.Events) == 0 {
		form.Events = append(form.Events, UserEvent{})
	}

	list, err := Dba.selectEventCategoryList(false)
	if err != nil {
		routeDbError(c, err)
		return
	}

	difficulties, err := Dba.selectDifficultyList(true)
	if err != nil {
		routeDbError(c, err)
		return
	}

	sessions, err := Dba.selectSessionList(true)
	if err != nil {
		routeDbError(c, err)
		return
	}

	times, err := Dba.selectTimeList(true)
	if err != nil {
		routeDbError(c, err)
		return
	}

	classes, err := Dba.selectClassList(true)
	if err != nil {
		routeDbError(c, err)
		return
	}

	cfg, err := Dba.selectConfig()
	if err != nil {
		routeDbError(c, err)
		return
	}

	tracksData, err := Dba.selectCacheTracks()
	if err != nil {
		routeDbError(c, err)
		return
	}
	tracksData = withDemoTracks(c, tracksData)

	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}

	c.HTML(http.StatusOK, "/htm/event.htm", gin.H{
		"page":          "event",
		"list":          list,
		"form":          form,
		"difficulties":  difficulties,
		"sessions":      sessions,
		"times":         times,
		"classes":       classes,
		"max_clients":   cfg.MaxClients,
		"track_data":    tracksData,
		"config_filled": cfgFilled,
		"status":        defaultStatus(),
	})
}

func routeDeleteEventCategory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := Dba.deleteEventCategory(id)

	if err != nil {
		routeFkError(c, err)
	} else {
		c.Redirect(http.StatusFound, "/event")
	}
}

func routeLogin(c *gin.Context) {
	if c.Request.Method == "POST" {
		usr := c.PostForm("name")
		pwd := c.PostForm("password")

		if debug && usr == "demo" && pwd == "demo" {
			claims := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"sub": "demo",
				"iss": "servermanager",
				"aud": "demo",
				"exp": time.Now().Add(time.Hour * 24 * 30).Unix(),
				"iat": time.Now().Unix(),
			})

			tokenString, err := claims.SignedString(SecretKey)
			if err != nil {
				c.String(http.StatusInternalServerError, "Error creating token")
				return
			}
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie("token", tokenString, 3600*24*30, "/", "", false, true)
			issueCsrfCookie(c)

			log.Print("Debug demo login successful")

			c.Redirect(http.StatusFound, "/")
			return
		}

		user, err := Dba.selectUser(usr)
		if err != nil {
			routeDbError(c, err)
			return
		}

		if user.Name == nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}
		res := bcrypt.CompareHashAndPassword([]byte(*user.Password), []byte(pwd))

		if res == nil {
			claims := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"sub": user.Name,
				"iss": "servermanager",
				"aud": "admin",
				"exp": time.Now().Add(time.Hour * 24 * 30).Unix(),
				"iat": time.Now().Unix(),
			})

			tokenString, err := claims.SignedString(SecretKey)
			if err != nil {
				c.String(http.StatusInternalServerError, "Error creating token")
				return
			}
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie("token", tokenString, 3600*24*30, "/", "", false, true)
			issueCsrfCookie(c)

			log.Print("Login successful for user: " + *user.Name)

			c.Redirect(http.StatusFound, "/")
			return
		}
	}

	c.HTML(http.StatusOK, "/htm/login.htm", gin.H{
		"debug": debug,
	})
}

func routeLogout(c *gin.Context) {
	c.SetCookie("token", "", 0, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

func routeUser(c *gin.Context) {

	user, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	form, err := Dba.selectUser(user.(string))
	if err != nil {
		if debug && user.(string) == "demo" {
			name := "demo"
			form = Users{
				Name: &name,
			}
		} else {
			routeDbError(c, err)
			return
		}
	}

	if user.(string) == "demo" {
		c.HTML(http.StatusOK, "/htm/user.htm", gin.H{
			"page":          "user",
			"form":          form,
			"config_filled": true,
			"status":        defaultStatus(),
		})
		return
	}

	if c.Request.Method == "POST" && c.ShouldBind(&form) == nil {
		_, err := Dba.updateUser(form)
		if err != nil {
			routeDbError(c, err)
			return
		}
	}

	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}

	c.HTML(http.StatusOK, "/htm/user.htm", gin.H{
		"page":          "user",
		"form":          form,
		"config_filled": cfgFilled,
		"status":        defaultStatus(),
	})
}

func routeAdmin(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	if user != "admin" {
		route403(c)
		return
	}

	cfgFilled, err := routeConfigFilled(c)
	if err != nil {
		routeDbError(c, err)
		return
	}

	c.HTML(http.StatusOK, "/htm/admin.htm", gin.H{
		"config_filled": cfgFilled,
		"status":        defaultStatus(),
	})

}

// Error routing

func routeDbError(c *gin.Context, err error) {
	git := "Hello! I've encountered the following error:\n\n```\n" + FormatError(err) + "\n```\n\nHere are the steps I was taking while this happened:\n(PLEASE FILL IN)"
	c.HTML(http.StatusInternalServerError, "/htm/error.htm", gin.H{
		"error":      "Application Error: " + err.Error(),
		"details":    FormatErrorHTML(err),
		"detailsGit": git,
		"title":      "500",
	})
}

func routeFkError(c *gin.Context, err error) {
	git := "Hello! I've encountered the following error:\n\n```\n" + FormatError(err) + "\n```\n\nHere are the steps I was taking while this happened:\n(PLEASE FILL IN)"
	c.HTML(http.StatusInternalServerError, "/htm/error.htm", gin.H{
		"error":      "Application Error: " + err.Error(),
		"details":    FormatErrorHTML(err),
		"detailsGit": git,
		"title":      "500",
	})
}

func route404(c *gin.Context) {
	c.HTML(http.StatusNotFound, "/htm/error.htm", gin.H{
		"error":   "Oops! Page not found",
		"details": "The page you are looking for might have been removed or is temporarily unavailable.",
		"title":   "404",
	})
}

func route403(c *gin.Context) {
	c.HTML(http.StatusForbidden, "/htm/error.htm", gin.H{
		"error":   "Forbidden",
		"details": "The server understood the request, but is refusing to authorize it.",
		"title":   "403",
	})
}
