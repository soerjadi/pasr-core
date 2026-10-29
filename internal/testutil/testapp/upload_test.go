package testapp

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"
	"strings"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/config"
	"github.com/oemah/nusantara/core/internal/testutil/contract"
)

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = uint8(i * 7)
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func errCode(t *testing.T, r *Resp) string {
	t.Helper()
	var e struct{ Code string }
	r.JSON(&e)
	return e.Code
}

func TestUpload_JPEG_ReturnsThreeVariants(t *testing.T) {
	env := New(t)
	s := env.SignIn("081234500001")

	// Client-supplied filename and content type are irrelevant; the bytes decide.
	r := env.DoFile(http.MethodPost, "/v1/me/images", "file", "../../x.txt", "text/plain", testJPEG(t, 900, 600), Bearer(s.AccessToken))

	if r.Status != http.StatusCreated {
		t.Fatalf("status = %d body = %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "uploadImage", r.Body)
	var up domain.ImageUpload
	r.JSON(&up)
	if up.ID == 0 {
		t.Fatal("missing id")
	}
	for url, wantW := range map[string]int{up.Image.Small: 320, up.Image.Medium: 640, up.Image.Large: 900} {
		key, ok := env.Storage.KeyFromURL(url)
		if !ok {
			t.Fatalf("url %q is not from storage", url)
		}
		data, ct, ok := env.Storage.Get(key)
		if !ok || ct != "image/jpeg" {
			t.Fatalf("object %s: ok=%v ct=%q", key, ok, ct)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width != wantW {
			t.Fatalf("variant %s width = %d, want %d (%v)", key, cfg.Width, wantW, err)
		}
		if strings.Contains(url, "x.txt") || strings.Contains(url, "..") {
			t.Fatalf("url derived from client input: %s", url)
		}
	}
}

func TestUpload_RequiresAuth(t *testing.T) {
	env := New(t)
	r := env.DoFile(http.MethodPost, "/v1/me/images", "file", "a.jpg", "image/jpeg", testJPEG(t, 100, 100))
	if r.Status != http.StatusUnauthorized {
		t.Fatalf("status = %d", r.Status)
	}
	if n := len(env.Storage.Keys()); n != 0 {
		t.Fatalf("%d objects stored for an anonymous request", n)
	}
}

func TestUpload_RenamedTextFileRejected(t *testing.T) {
	env := New(t)
	s := env.SignIn("081234500001")
	r := env.DoFile(http.MethodPost, "/v1/me/images", "file", "photo.jpg", "image/jpeg", []byte("just some text"), Bearer(s.AccessToken))
	if r.Status != http.StatusBadRequest || errCode(t, r) != "IMAGE_INVALID" {
		t.Fatalf("status = %d body = %s", r.Status, r.Body)
	}
}

func TestUpload_OverMaxBytesRejected(t *testing.T) {
	env := New(t, WithConfig(func(c *config.Config) { c.Images.MaxBytes = 50_000 }))
	s := env.SignIn("081234500001")

	// Slightly over the limit: the service rejects it.
	r := env.DoFile(http.MethodPost, "/v1/me/images", "file", "a.jpg", "image/jpeg", bytes.Repeat([]byte{0xFF}, 60_000), Bearer(s.AccessToken))
	if r.Status != http.StatusBadRequest || errCode(t, r) != "IMAGE_TOO_LARGE" {
		t.Fatalf("status = %d body = %s", r.Status, r.Body)
	}
	// Far over: the request body cap stops reading early and gives the same answer.
	r = env.DoFile(http.MethodPost, "/v1/me/images", "file", "a.jpg", "image/jpeg", bytes.Repeat([]byte{0xFF}, 3_000_000), Bearer(s.AccessToken))
	if r.Status != http.StatusBadRequest || errCode(t, r) != "IMAGE_TOO_LARGE" {
		t.Fatalf("huge: status = %d body = %s", r.Status, r.Body)
	}
}

func TestUpload_MissingFileField(t *testing.T) {
	env := New(t)
	s := env.SignIn("081234500001")
	r := env.DoFile(http.MethodPost, "/v1/me/images", "other", "a.jpg", "image/jpeg", testJPEG(t, 50, 50), Bearer(s.AccessToken))
	if r.Status != http.StatusBadRequest || errCode(t, r) != "IMAGE_INVALID" {
		t.Fatalf("status = %d body = %s", r.Status, r.Body)
	}
	r = env.Do(http.MethodPost, "/v1/me/images", json.RawMessage(`{}`), Bearer(s.AccessToken))
	if r.Status != http.StatusBadRequest || errCode(t, r) != "IMAGE_INVALID" {
		t.Fatalf("non-multipart: status = %d body = %s", r.Status, r.Body)
	}
}
