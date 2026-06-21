package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RoleMiddleware enforces capabilities centrally, by HTTP method and path,
// after authentication. admin = everything; steward = operate running servers
// and queues; viewer = read-only. Backend is the source of truth — the SPA
// only hides controls.
func RoleMiddleware(c *gin.Context) {
	method := c.Request.Method
	path := c.Request.URL.Path
	roleVal, _ := c.Get("role")
	role, _ := roleVal.(string)

	// The Users admin area is admin-only, reads included.
	if strings.HasPrefix(path, "/api/users") && role != roleAdmin {
		forbidden(c)
		return
	}

	// Stream debug exposes capture URLs (which can carry tokens) and can run an
	// ffmpeg probe — admin-only, reads included.
	if strings.HasPrefix(path, "/api/streams/debug") && role != roleAdmin {
		forbidden(c)
		return
	}

	// Safe methods, logout, and editing your own account are open to any role.
	if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions ||
		path == "/api/logout" || path == "/api/user" {
		c.Next()
		return
	}

	switch role {
	case roleAdmin:
		c.Next()
	case roleSteward:
		if stewardCanMutate(path) {
			c.Next()
			return
		}
		forbidden(c)
	default:
		forbidden(c)
	}
}

// stewardCanMutate whitelists the operate-time mutations a steward may perform:
// running-server commands, the queue, per-instance run mode / schedule, and
// car-class presets.
func stewardCanMutate(path string) bool {
	p := strings.TrimPrefix(path, "/api")
	switch {
	case strings.HasPrefix(p, "/server/"):
		return true
	case strings.HasPrefix(p, "/queue"):
		return true
	case strings.HasPrefix(p, "/instances/") && (strings.HasSuffix(p, "/runmode") || strings.HasSuffix(p, "/schedule")):
		return true
	case p == "/classes" || strings.HasPrefix(p, "/class/"):
		return true
	case strings.HasPrefix(p, "/drivers/") && (strings.HasSuffix(p, "/avatar") || strings.Contains(p, "/record") || strings.HasSuffix(p, "/snapshot")):
		// Uploading a driver photo, starting/stopping a manual recording, and
		// grabbing a snapshot are operate-time actions.
		return true
	case strings.HasPrefix(p, "/drivers/") && strings.Contains(p, "/media/"):
		// Deleting a highlight clip/screenshot is an operate-time action.
		return true
	case strings.HasPrefix(p, "/drivers/") && strings.Contains(p, "/sessions/") && strings.Contains(p, "/tags"):
		// Tagging a driver's session (for later search) is an operate-time action.
		return true
	case p == "/guest-drivers" || strings.HasPrefix(p, "/guest-drivers/"):
		// Managing the shared-account roster (incl. avatar upload) is operate-time.
		return true
	case strings.HasPrefix(p, "/scores/") && strings.HasSuffix(p, "/assign"):
		// Re-attributing a leaderboard row to the right person is operate-time.
		return true
	}
	return false
}

func forbidden(c *gin.Context) {
	apiError(c, http.StatusForbidden, "forbidden", "Your role does not allow this action.")
	c.Abort()
}
