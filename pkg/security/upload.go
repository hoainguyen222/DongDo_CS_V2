package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// ValidateAndSanitizeFilename removes path traversal attempts and dangerous characters.
// Returns the sanitized filename or an error.
func ValidateAndSanitizeFilename(filename string) (string, error) {
	// 1. Decode URL-encoded characters (e.g. %2F for /)
	decoded, err := url.QueryUnescape(filename)
	if err != nil {
		return "", fmt.Errorf("invalid filename encoding")
	}

	// 2. Reject any path traversal attempts in the raw decoded name
	//    (must be checked BEFORE filepath.Base, since filepath.Base("../../../etc/passwd") == "passwd")
	if strings.Contains(decoded, "..") {
		return "", fmt.Errorf("path traversal attempt detected")
	}

	// 3. Take only the base (last path element) — also rejects absolute paths
	name := filepath.Base(decoded)

	// 4. Re-check after Base for any remaining ".."
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("path traversal attempt detected")
	}

	// 5. Allow only safe characters: alphanumeric, dash, underscore, dot, space, Vietnamese Unicode letters
	var clean strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' || r == ' ' {
			clean.WriteRune(r)
		} else {
			return "", fmt.Errorf("filename contains invalid character: %c", r)
		}
	}

	result := strings.TrimSpace(clean.String())
	if result == "" || result == "." {
		return "", fmt.Errorf("empty filename after sanitization")
	}

	return result, nil
}

// CheckPrefix ensures safePath is inside allowedDir.
// Returns error if safePath escapes the allowed directory.
func CheckPrefix(allowedDir, safePath string) error {
	absAllowed, err := filepath.Abs(allowedDir)
	if err != nil {
		return fmt.Errorf("cannot resolve allowed directory: %w", err)
	}
	absPath, err := filepath.Abs(safePath)
	if err != nil {
		return fmt.Errorf("cannot resolve file path: %w", err)
	}
	// filepath.Clean normalizes the path (removes trailing slash, resolves . and ..)
	absAllowed = filepath.Clean(absAllowed)
	absPath = filepath.Clean(absPath)
	if !strings.HasPrefix(absPath, absAllowed) {
		return fmt.Errorf("file path escapes allowed directory")
	}
	return nil
}

// GenerateSecureFilename creates a server-generated filename with a random component.
// The user-provided filename is never used as the stored filename.
func GenerateSecureFilename(prefix, extension string) (string, error) {
	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return fmt.Sprintf("%s_%s_%d.%s",
		prefix,
		hex.EncodeToString(randBytes),
		time.Now().UnixMilli(),
		extension,
	), nil
}

// ValidateDOCXMagicBytes checks that a .docx file has the expected ZIP (PK) magic bytes.
// .docx files are ZIP archives under the hood.
func ValidateDOCXMagicBytes(r io.ReadSeeker) error {
	header := make([]byte, 4)
	if _, err := r.Read(header); err != nil {
		return fmt.Errorf("cannot read file header")
	}
	// Seek back to beginning so the caller can re-read
	r.Seek(0, io.SeekStart)
	// PK.. = ZIP signature (0x50=P, 0x4B=K)
	if header[0] == 0x50 && header[1] == 0x4B && header[2] == 0x03 && header[3] == 0x04 {
		return nil
	}
	return fmt.Errorf("file is not a valid .docx (expected ZIP format with PK signature)")
}

// ValidateAudioMagicBytes checks that an audio file has plausible audio data
// by reading the first few bytes. This is a basic check, not a full format validator.
func ValidateAudioMagicBytes(r io.ReadSeeker) error {
	header := make([]byte, 8)
	n, err := r.Read(header)
	if err != nil && err != io.EOF {
		return fmt.Errorf("cannot read file header: %w", err)
	}
	r.Seek(0, io.SeekStart)
	// Audio magic bytes:
	// .webm / .ogg: starts with OggS (0x4F 0x67 0x67 0x53)
	// .wav: starts with RIFF (0x52 0x49 0x46 0x46)
	// .mp3: starts with ID3 (0x49 0x44 0x33) or MPG (0xFF 0xFB)
	// .m4a: starts with ftyp (0x66 0x74 0x79 0x70)
	valid := false
	if n >= 4 {
		valid = valid ||
			(header[0] == 0x4F && header[1] == 0x67 && header[2] == 0x67 && header[3] == 0x53) || // OggS
			(header[0] == 0x52 && header[1] == 0x49 && header[2] == 0x46 && header[3] == 0x46) || // RIFF/WAV
			(header[0] == 0x49 && header[1] == 0x44 && header[2] == 0x33) || // ID3/MP3
			(header[0] == 0xFF && (header[1]&0xE0) == 0xE0) || // MP3 frame sync
			(header[0] == 0x66 && header[1] == 0x74 && header[2] == 0x79 && header[3] == 0x70) || // ftyp/M4A
			(header[0] == 0x23 && header[1] == 0x21) // shebang fallback for some webms
	}
	if !valid {
		return fmt.Errorf("file does not appear to be a valid audio file")
	}
	return nil
}

// Upload limits
const (
	MaxDocUploadSize   = 50 * 1024 * 1024  // 50 MB
	MaxAudioUploadSize = 100 * 1024 * 1024 // 100 MB
)

// AllowedDOCXMIMETypes is a whitelist of MIME types accepted for document uploads.
var AllowedDOCXMIMETypes = map[string]bool{
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
}

// AllowedAudioExtensions is a whitelist of allowed audio file extensions.
var AllowedAudioExtensions = map[string]bool{
	".webm": true,
	".ogg":  true,
	".wav":  true,
	".mp3":  true,
	".m4a":  true,
}

// EnsureDirExists creates the directory if it does not exist.
func EnsureDirExists(dir string) error {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	if err != nil {
		return fmt.Errorf("cannot stat directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path exists but is not a directory: %s", dir)
	}
	return nil
}
