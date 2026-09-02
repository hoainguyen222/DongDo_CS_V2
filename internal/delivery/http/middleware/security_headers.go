package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders adds HTTP security headers to every response.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Strict-Transport-Security: enable only when HTTPS is detected.
		// Set by reverse proxy (nginx/caddy) in production.
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			c.Writer.Header().Set("Strict-Transport-Security",
				"max-age=31536000; includeSubDomains; preload")
		}

		// Prevent MIME-type sniffing
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking — deny framing from any origin
		c.Writer.Header().Set("X-Frame-Options", "DENY")

		// Control referrer information sent to third parties
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Restrict browser feature permissions
		c.Writer.Header().Set("Permissions-Policy",
			"camera=(), microphone=(), geolocation=(), payment=()")

		// Content-Security-Policy: allows self + inline scripts (required by Next.js)
		c.Writer.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' 'unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https: blob:; "+
				"font-src 'self' data:; "+
				"connect-src 'self' ws: wss: https:; "+
				"media-src 'self' blob:; "+
				"object-src 'none'; "+
				"frame-ancestors 'none'")

		// X-XSS-Protection (legacy, but some browsers still check it)
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")

		c.Next()
	}
}
