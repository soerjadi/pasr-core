package catalog

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type memStore struct {
	mu   sync.Mutex
	objs map[string][]byte
	ct   map[string]string
}

func newMemStore() *memStore { return &memStore{objs: map[string][]byte{}, ct: map[string]string{}} }

func (m *memStore) Put(_ context.Context, key, contentType string, r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objs[key] = b
	m.ct[key] = contentType
	return nil
}

func (m *memStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objs, key)
	return nil
}

func (m *memStore) URL(key string) string { return "https://media.test/" + key }

type fakeUploads struct {
	mu       sync.Mutex
	rows     map[int64]domain.ImageKeys
	next     int64
	orphans  []domain.ImageKeys
	detached []domain.ImageKeys
	cutoff   time.Time
	dcutoff  time.Time
}

func (f *fakeUploads) Create(_ context.Context, _ int64, k domain.ImageKeys) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rows == nil {
		f.rows = map[int64]domain.ImageKeys{}
	}
	f.next++
	f.rows[f.next] = k
	return f.next, nil
}
func (f *fakeUploads) ResolveOwned(context.Context, int64, []int64) (map[int64]domain.ImageKeys, error) {
	return nil, nil
}
func (f *fakeUploads) MarkAttached(context.Context, []int64) error { return nil }
func (f *fakeUploads) DeleteOrphans(_ context.Context, olderThan time.Time) ([]domain.ImageKeys, error) {
	f.cutoff = olderThan
	return f.orphans, nil
}
func (f *fakeUploads) DeleteDetached(_ context.Context, olderThan time.Time) ([]domain.ImageKeys, error) {
	f.dcutoff = olderThan
	return f.detached, nil
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func newSvc(maxBytes, maxPixels int64) (*service, *fakeUploads, *memStore) {
	up, st := &fakeUploads{}, newMemStore()
	s := NewService(fakeCategories{}, nil, nil, nil, nil, up, st, Config{MaxImageBytes: maxBytes, MaxImagePixels: maxPixels}, fixedClock{time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)})
	return s.(*service), up, st
}

func solid(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 90, 255})
		}
	}
	return img
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, solid(w, h), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, solid(w, h)); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// 120x80 lossless WebP (no WebP encoder exists in the Go ecosystem we depend on).
const webp120x80 = "UklGRiQAAABXRUJQVlA4TBcAAAAvd8ATAAdQjyLXo/9hABLC//1SRP9TVwA="

func (m *memStore) size(t *testing.T, key string) (int, int) {
	t.Helper()
	cfg, format, err := image.DecodeConfig(bytes.NewReader(m.objs[key]))
	if err != nil || format != "jpeg" {
		t.Fatalf("object %s: format %q err %v", key, format, err)
	}
	return cfg.Width, cfg.Height
}

func upload(t *testing.T, s *service, b []byte) (domain.ImageUpload, error) {
	t.Helper()
	return s.UploadImage(context.Background(), 7, bytes.NewReader(b))
}

func TestUpload_JPEG_ReturnsThreeVariants(t *testing.T) {
	for _, tc := range []struct {
		name string
		w    int
		want [3]int
	}{
		{"2000px source", 2000, [3]int{320, 640, 1080}},
		{"500px source is never upscaled", 500, [3]int{320, 500, 500}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, up, st := newSvc(10<<20, 40_000_000)
			res, err := upload(t, s, jpegBytes(t, tc.w, tc.w/2))
			if err != nil {
				t.Fatal(err)
			}
			keys := up.rows[res.ID]
			for i, k := range []string{keys.Small, keys.Medium, keys.Large} {
				if w, _ := st.size(t, k); w != tc.want[i] {
					t.Errorf("variant %d width = %d, want %d", i, w, tc.want[i])
				}
				if st.ct[k] != "image/jpeg" {
					t.Errorf("content type %q", st.ct[k])
				}
			}
			if res.Image.Small != "https://media.test/"+keys.Small || res.Image.Large != "https://media.test/"+keys.Large {
				t.Fatalf("image urls = %+v keys = %+v", res.Image, keys)
			}
		})
	}
}

func TestUpload_KeysAreRandomAndOpaque(t *testing.T) {
	s, up, _ := newSvc(10<<20, 40_000_000)
	a, _ := upload(t, s, jpegBytes(t, 400, 300))
	b, _ := upload(t, s, jpegBytes(t, 400, 300))
	ka, kb := up.rows[a.ID], up.rows[b.ID]
	if ka.Small == kb.Small || ka.Large == kb.Large {
		t.Fatal("keys repeat across uploads")
	}
	for _, k := range ka.All() {
		if !strings.HasPrefix(k, "images/") || !strings.HasSuffix(k, ".jpg") || strings.Contains(k, "..") {
			t.Fatalf("unexpected key %q", k)
		}
	}
}

func TestUpload_PNGAndWebP(t *testing.T) {
	s, up, st := newSvc(10<<20, 40_000_000)

	res, err := upload(t, s, pngBytes(t, 700, 300))
	if err != nil {
		t.Fatalf("png: %v", err)
	}
	if w, _ := st.size(t, up.rows[res.ID].Medium); w != 640 {
		t.Fatalf("png medium width = %d", w)
	}

	raw, _ := base64.StdEncoding.DecodeString(webp120x80)
	res, err = upload(t, s, raw)
	if err != nil {
		t.Fatalf("webp: %v", err)
	}
	if w, h := st.size(t, up.rows[res.ID].Large); w != 120 || h != 80 {
		t.Fatalf("webp large = %dx%d", w, h)
	}
}

func TestUpload_RenamedTextFileRejected(t *testing.T) {
	s, up, _ := newSvc(10<<20, 40_000_000)
	for name, body := range map[string][]byte{
		"text":  []byte("this is definitely not an image.jpg"),
		"empty": {},
		"html":  []byte("<html><script>alert(1)</script></html>"),
		"gif":   []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
	} {
		_, err := upload(t, s, body)
		var ce *domain.CodedError
		if !errors.As(err, &ce) || ce.Code != "IMAGE_INVALID" || !errors.Is(err, domain.ErrValidationFailed) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if len(up.rows) != 0 {
		t.Fatal("rejected files must not create rows")
	}
}

func TestUpload_TruncatedImageRejected(t *testing.T) {
	s, _, _ := newSvc(10<<20, 40_000_000)
	b := jpegBytes(t, 400, 300)
	_, err := upload(t, s, b[:len(b)/2])
	var ce *domain.CodedError
	if !errors.As(err, &ce) || ce.Code != "IMAGE_INVALID" {
		t.Fatalf("err = %v", err)
	}
}

func TestUpload_OverMaxBytesRejected(t *testing.T) {
	s, up, _ := newSvc(1000, 40_000_000)
	_, err := upload(t, s, jpegBytes(t, 800, 600))
	var ce *domain.CodedError
	if !errors.As(err, &ce) || ce.Code != "IMAGE_TOO_LARGE" || !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("err = %v", err)
	}
	if len(up.rows) != 0 {
		t.Fatal("row created for oversize upload")
	}
}

// bombPNG is a valid PNG signature and IHDR claiming 60000x60000 pixels with no image data:
// DecodeConfig accepts it, a full decode would fail (or allocate ~14 GB first).
func bombPNG() []byte {
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], 60000)
	binary.BigEndian.PutUint32(ihdr[4:], 60000)
	ihdr[8], ihdr[9] = 8, 6 // 8-bit RGBA
	chunk := append([]byte("IHDR"), ihdr...)
	_ = binary.Write(&b, binary.BigEndian, uint32(13))
	b.Write(chunk)
	_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return b.Bytes()
}

func TestUpload_PixelBombRejectedWithoutDecoding(t *testing.T) {
	s, up, _ := newSvc(10<<20, 40_000_000)
	_, err := upload(t, s, bombPNG())
	var ce *domain.CodedError
	if !errors.As(err, &ce) || ce.Code != "IMAGE_INVALID" {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(ce.Message, "pixels") {
		t.Fatalf("expected the pixel-limit rejection, got %q (a decode failure would also say invalid)", ce.Message)
	}
	if len(up.rows) != 0 {
		t.Fatal("row created")
	}
}

// withOrientation inserts an EXIF APP1 segment with the given orientation right after SOI.
func withOrientation(t *testing.T, jpg []byte, orientation uint16) []byte {
	t.Helper()
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, byte(orientation >> 8), byte(orientation), 0, 0, 0, 0, 0, 0}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	seg = append(seg, payload...)
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	return append(out, jpg[2:]...)
}

func TestUpload_ExifRotationApplied(t *testing.T) {
	s, up, st := newSvc(10<<20, 40_000_000)
	// Stored landscape 1600x800 with orientation 6 (rotate 90 CW) displays as portrait 800x1600.
	res, err := upload(t, s, withOrientation(t, jpegBytes(t, 1600, 800), 6))
	if err != nil {
		t.Fatal(err)
	}
	w, h := st.size(t, up.rows[res.ID].Large)
	if w != 800 || h != 1600 {
		t.Fatalf("large = %dx%d, want 800x1600 (rotated)", w, h)
	}
	// Control: no EXIF stays landscape.
	res, _ = upload(t, s, jpegBytes(t, 1600, 800))
	if w, h := st.size(t, up.rows[res.ID].Large); w != 1080 || h != 540 {
		t.Fatalf("control large = %dx%d", w, h)
	}
}

func TestUpload_AlphaIsFlattenedOnWhite(t *testing.T) {
	s, up, st := newSvc(10<<20, 40_000_000)
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100)) // fully transparent
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	res, err := upload(t, s, b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	out, err := jpeg.Decode(bytes.NewReader(st.objs[up.rows[res.ID].Small]))
	if err != nil {
		t.Fatal(err)
	}
	r, g, bl, _ := out.At(50, 50).RGBA()
	if r>>8 < 240 || g>>8 < 240 || bl>>8 < 240 {
		t.Fatalf("transparent pixel became %d,%d,%d, want white", r>>8, g>>8, bl>>8)
	}
}

func TestOrphanGC_DeletesOldUnattachedOnly(t *testing.T) {
	s, up, st := newSvc(10<<20, 40_000_000)
	k := domain.ImageKeys{Small: "images/x-s.jpg", Medium: "images/x-m.jpg", Large: "images/x-l.jpg"}
	for _, key := range k.All() {
		st.objs[key] = []byte("x")
	}
	d := domain.ImageKeys{Small: "images/d-s.jpg", Medium: "images/d-m.jpg", Large: "images/d-l.jpg"}
	for _, key := range d.All() {
		st.objs[key] = []byte("d")
	}
	st.objs["images/keep.jpg"] = []byte("y")
	up.orphans = []domain.ImageKeys{k}
	up.detached = []domain.ImageKeys{d}

	n, err := s.PurgeOrphanUploads(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("n = %d err = %v", n, err)
	}
	if want := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC); !up.cutoff.Equal(want) || !up.dcutoff.Equal(want) {
		t.Fatalf("cutoffs = %v %v, want 24h before now (%v)", up.cutoff, up.dcutoff, want)
	}
	if len(st.objs) != 1 || st.objs["images/keep.jpg"] == nil {
		t.Fatalf("objects left: %v", st.objs)
	}
}
