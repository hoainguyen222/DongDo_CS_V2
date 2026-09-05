package asterisk

import (
	"os"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/rs/zerolog"
)

// Config carries all settings needed by the AMI client. Values default to
// safe production choices; callers are expected to fill Enabled correctly.
//
// All durations are kept in this file so the AMI client code stays focused on
// protocol mechanics.
type Config struct {
	Host     string        // AMI host (e.g. "127.0.0.1")
	Port     int           // AMI TCP port, defaults to 5038
	Username string        // AMI user (typically "dongdo")
	Password string        // AMI secret (configured in /etc/asterisk/manager.conf)
	Context  string        // Default dialplan context (e.g. "from-internal")
	Trunk    string        // Outbound trunk technology/resource (e.g. "PJSIP/trunk")
	Queue    string        // Agent queue name for inbound call distribution

	// Reconnection tuning - all have safe defaults.
	ReconnectMin      time.Duration // Initial backoff (default 1s)
	ReconnectMax      time.Duration // Max backoff (default 30s)
	DialTimeout       time.Duration // TCP connect timeout (default 5s)
	ReadTimeout       time.Duration // Socket read deadline (default 60s)
	WriteTimeout      time.Duration // Socket write deadline (default 5s)
	LoginTimeout      time.Duration // Time to wait for Login response (default 5s)
	EventBufferSize   int           // Bounded channel for CallEvent (default 512)
	OriginateTimeout  time.Duration // Synchronous originate wait (default 30s)
}

// DefaultConfig returns a Config populated with production defaults. The
// caller must still provide Host, Port, Username and Password.
func DefaultConfig() Config {
	return Config{
		Host:             "localhost",
		Port:             5038,
		Username:         "dongdo",
		Password:         "",
		Context:          "from-internal",
		Trunk:            "PJSIP/trunk",
		Queue:            "dongdo-queue",
		ReconnectMin:     1 * time.Second,
		ReconnectMax:     30 * time.Second,
		DialTimeout:      5 * time.Second,
		ReadTimeout:      60 * time.Second,
		WriteTimeout:     5 * time.Second,
		LoginTimeout:     5 * time.Second,
		EventBufferSize:  512,
		OriginateTimeout: 30 * time.Second,
	}
}

// FromDomainConfig converts the config.AsteriskConfig struct exposed by the
// application config package into the infra-local Config with sensible
// defaults filled in.
func FromDomainConfig(d domain.AsteriskConfig) Config {
	c := DefaultConfig()

	if d.Host != "" {
		c.Host = d.Host
	}
	if d.Port > 0 {
		c.Port = d.Port
	}
	if d.Username != "" {
		c.Username = d.Username
	}
	if d.Password != "" {
		c.Password = d.Password
	} else {
		// Fall back to environment so credentials never have to live in code.
		if env := os.Getenv("ASTERISK_PASS"); env != "" {
			c.Password = env
		}
	}
	if d.Context != "" {
		c.Context = d.Context
	}
	if d.Trunk != "" {
		c.Trunk = d.Trunk
	}
	if d.Queue != "" {
		c.Queue = d.Queue
	}

	return c
}

// Validate returns an error if the config cannot drive a working AMI client.
// Enabled callers should treat validation failures as a fatal misconfig.
func (c Config) Validate() error {
	if c.Host == "" {
		return errInvalid("host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return errInvalid("port must be in 1..65535")
	}
	if c.Username == "" {
		return errInvalid("username is required")
	}
	if c.Password == "" {
		return errInvalid("password is required")
	}
	if c.Context == "" {
		return errInvalid("context is required")
	}
	return nil
}

func errInvalid(reason string) error {
	return &configError{reason: reason}
}

type configError struct{ reason string }

func (e *configError) Error() string { return "asterisk config invalid: " + e.reason }

// IsConfigError reports whether err originated from Config.Validate.
func IsConfigError(err error) bool {
	_, ok := err.(*configError)
	return ok
}

// newLogger returns a package-local zerolog logger with the standard
// "component" tag. Tests can override with SetLogger.
func newLogger() zerolog.Logger {
	return zerolog.New(os.Stderr).With().Timestamp().Logger()
}
