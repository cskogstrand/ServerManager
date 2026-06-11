package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// apiError is the error envelope all new JSON endpoints use:
// {"error": {"code": "...", "message": "..."}}
// Legacy endpoints keep their {"success": false, "message": ...} shape until
// the old UI is retired.
func apiError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}

// apiDbError maps DB errors to the envelope; foreign-key violations become a
// 409 because they mean "still referenced by something else".
func apiDbError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "FOREIGN KEY") {
		apiError(c, http.StatusConflict, "in_use", "This item is still used by an event or queue entry and cannot be deleted.")
		return
	}
	apiError(c, http.StatusInternalServerError, "db_error", err.Error())
}

func apiBadRequest(c *gin.Context, message string) {
	apiError(c, http.StatusBadRequest, "bad_request", message)
}

func apiNotFound(c *gin.Context) {
	apiError(c, http.StatusNotFound, "not_found", "Resource not found")
}

// pathId parses the :id path parameter and replies 400 on garbage.
func pathId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		apiBadRequest(c, "Invalid id")
		return 0, false
	}
	return id, true
}

const csrfCookieName = "csrf_token"
const csrfHeaderName = "X-CSRF-Token"

func newCsrfToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}

// issueCsrfCookie sets the double-submit cookie. Not HttpOnly on purpose:
// the frontend reads it and mirrors it into the X-CSRF-Token header.
func issueCsrfCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(csrfCookieName, newCsrfToken(), 3600*24*30, "/", "", false, false)
}

// issueAuthCookies sets the JWT session cookie plus the CSRF cookie.
// Shared by the HTML form login and the SPA JSON login.
func issueAuthCookies(c *gin.Context, sub string, aud string) error {
	claims := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": sub,
		"iss": "servermanager",
		"aud": aud,
		"exp": time.Now().Add(time.Hour * 24 * 30).Unix(),
		"iat": time.Now().Unix(),
	})

	tokenString, err := claims.SignedString(SecretKey)
	if err != nil {
		return err
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", tokenString, 3600*24*30, "/", "", false, true)
	issueCsrfCookie(c)
	return nil
}

// CsrfMiddleware enforces double-submit on mutating API requests: the
// X-CSRF-Token header must match the csrf_token cookie.
func CsrfMiddleware(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		// Sessions from before the CSRF rollout have a valid JWT but no
		// csrf cookie yet — hand one out on the first safe request.
		if cookie, err := c.Cookie(csrfCookieName); err != nil || cookie == "" {
			issueCsrfCookie(c)
		}
		c.Next()
		return
	}

	cookie, err := c.Cookie(csrfCookieName)
	header := c.GetHeader(csrfHeaderName)
	if err != nil || cookie == "" || header == "" || subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) != 1 {
		apiError(c, http.StatusForbidden, "csrf", "Missing or invalid CSRF token. Reload the page and try again.")
		c.Abort()
		return
	}

	c.Next()
}
