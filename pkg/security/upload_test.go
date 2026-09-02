package security

import (
	"bytes"
	"testing"
)

// ─── ValidateAndSanitizeFilename tests ─────────────────────────────────────

func TestValidateAndSanitizeFilename_Normal(t *testing.T) {
	got, err := ValidateAndSanitizeFilename("document.docx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "document.docx" {
		t.Errorf("expected document.docx, got %s", got)
	}
}

func TestValidateAndSanitizeFilename_WithSpaces(t *testing.T) {
	got, err := ValidateAndSanitizeFilename("my document file.docx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "my document file.docx" {
		t.Errorf("expected 'my document file.docx', got %s", got)
	}
}

func TestValidateAndSanitizeFilename_URLEncoded(t *testing.T) {
	// %2F = /, %2E = .
	got, err := ValidateAndSanitizeFilename("file%2Edocx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "file.docx" {
		t.Errorf("expected file.docx, got %s", got)
	}
}

func TestValidateAndSanitizeFilename_PathTraversal(t *testing.T) {
	// These should all error: contain ".." anywhere
	badCases := []string{
		"../../../etc/passwd",
		"..%2F..%2Fetc%2Fpasswd",
		"foo/../etc/passwd",
		"a..b.docx", // contains ".."
	}
	for _, tc := range badCases {
		_, err := ValidateAndSanitizeFilename(tc)
		if err == nil {
			t.Errorf("expected error for path traversal %q, got nil", tc)
		}
	}

	// /etc/passwd → filepath.Base extracts "passwd" which is harmless.
	// The function intentionally strips the path prefix.
	got, err := ValidateAndSanitizeFilename("/etc/passwd")
	if err != nil {
		t.Errorf("expected no error for absolute path /etc/passwd (extracted to base 'passwd'), got %v", err)
	}
	if got != "passwd" {
		t.Errorf("expected 'passwd', got %s", got)
	}
}

func TestValidateAndSanitizeFilename_InvalidChars(t *testing.T) {
	_, err := ValidateAndSanitizeFilename("file<>:*.docx")
	if err == nil {
		t.Error("expected error for invalid chars, got nil")
	}
}

func TestValidateAndSanitizeFilename_Empty(t *testing.T) {
	_, err := ValidateAndSanitizeFilename("")
	if err == nil {
		t.Error("expected error for empty filename, got nil")
	}
}

func TestValidateAndSanitizeFilename_DotOnly(t *testing.T) {
	_, err := ValidateAndSanitizeFilename(".")
	if err == nil {
		t.Error("expected error for dot-only filename, got nil")
	}
}

func TestValidateAndSanitizeFilename_UnicodeVietnamese(t *testing.T) {
	// Vietnamese characters should be allowed (they are Unicode letters)
	got, err := ValidateAndSanitizeFilename("tài_liệu_thông_tin.docx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "tài_liệu_thông_tin.docx" {
		t.Errorf("expected original, got %s", got)
	}
}

// ─── CheckPrefix tests ─────────────────────────────────────────────────────

func TestCheckPrefix_ValidPath(t *testing.T) {
	err := CheckPrefix("/tmp/uploads", "/tmp/uploads/file.docx")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCheckPrefix_Escape(t *testing.T) {
	err := CheckPrefix("/tmp/uploads", "/tmp/uploads/../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path escape, got nil")
	}
}

func TestCheckPrefix_SamePath(t *testing.T) {
	err := CheckPrefix("/tmp/uploads", "/tmp/uploads")
	if err != nil {
		t.Errorf("expected no error for identical path, got %v", err)
	}
}

func TestCheckPrefix_Subdirectory(t *testing.T) {
	err := CheckPrefix("/tmp/uploads", "/tmp/uploads/subdir/file.docx")
	if err != nil {
		t.Errorf("expected no error for subdirectory, got %v", err)
	}
}

// ─── GenerateSecureFilename tests ──────────────────────────────────────────

func TestGenerateSecureFilename_Unique(t *testing.T) {
	name1, err := GenerateSecureFilename("call", "webm")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	name2, err := GenerateSecureFilename("call", "webm")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if name1 == name2 {
		t.Error("expected different filenames for consecutive calls")
	}
}

func TestGenerateSecureFilename_HasPrefix(t *testing.T) {
	name, err := GenerateSecureFilename("call", "wav")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !bytes.HasPrefix([]byte(name), []byte("call_")) {
		t.Errorf("expected prefix 'call_', got %s", name)
	}
	if !bytes.HasSuffix([]byte(name), []byte(".wav")) {
		t.Errorf("expected suffix '.wav', got %s", name)
	}
}

// ─── ValidateDOCXMagicBytes tests ──────────────────────────────────────────

func TestValidateDOCXMagicBytes_ValidDOCX(t *testing.T) {
	// PK.. = ZIP signature
	data := []byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00}
	err := ValidateDOCXMagicBytes(bytes.NewReader(data))
	if err != nil {
		t.Errorf("expected no error for valid ZIP/DOCX, got %v", err)
	}
}

func TestValidateDOCXMagicBytes_NotAZIP(t *testing.T) {
	// Random bytes that don't look like a ZIP
	data := []byte("Hello World")
	err := ValidateDOCXMagicBytes(bytes.NewReader(data))
	if err == nil {
		t.Error("expected error for non-DOCX file, got nil")
	}
}

func TestValidateDOCXMagicBytes_ShortRead(t *testing.T) {
	// Fewer than 4 bytes
	data := []byte{0x50, 0x4B}
	err := ValidateDOCXMagicBytes(bytes.NewReader(data))
	if err == nil {
		t.Error("expected error for short read, got nil")
	}
}

// ─── ValidateAudioMagicBytes tests ─────────────────────────────────────────

func TestValidateAudioMagicBytes_WAV(t *testing.T) {
	// RIFF....WAVE
	data := []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x41, 0x56, 0x45}
	err := ValidateAudioMagicBytes(bytes.NewReader(data))
	if err != nil {
		t.Errorf("expected no error for WAV, got %v", err)
	}
}

func TestValidateAudioMagicBytes_OGG(t *testing.T) {
	// OggS
	data := []byte{0x4F, 0x67, 0x67, 0x53, 0x00, 0x02, 0x00, 0x00}
	err := ValidateAudioMagicBytes(bytes.NewReader(data))
	if err != nil {
		t.Errorf("expected no error for OGG, got %v", err)
	}
}

func TestValidateAudioMagicBytes_Invalid(t *testing.T) {
	// Random binary
	data := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}
	err := ValidateAudioMagicBytes(bytes.NewReader(data))
	if err == nil {
		t.Error("expected error for invalid audio, got nil")
	}
}
