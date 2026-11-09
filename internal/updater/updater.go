package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const Version = "0.1.0"

type Release struct {
	Version   string `json:"version"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature"`
	Notes     string `json:"notes"`
}
type Channel struct {
	URL       string `json:"url"`
	PublicKey string `json:"publicKey"`
}

func httpsURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" || u.Host == "" || u.User != nil {
		return errors.New("Kênh cập nhật phải dùng HTTPS")
	}
	return nil
}
func client() *http.Client {
	return &http.Client{Timeout: 15 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return errors.New("Quá nhiều redirect")
		}
		return httpsURL(r.URL.String())
	}}
}
func ValidateChannel(channel Channel) error {
	if channel.URL == "" && channel.PublicKey == "" {
		return nil
	}
	if err := httpsURL(channel.URL); err != nil {
		return err
	}
	key, err := base64.StdEncoding.DecodeString(channel.PublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("Public key Ed25519 cần base64 của 32 byte")
	}
	return nil
}
func Verify(r Release, keyBase64 string) error {
	if !regexp.MustCompile(`^\d{1,6}\.\d{1,6}\.\d{1,6}$`).MatchString(r.Version) {
		return errors.New("Version cần dạng x.y.z")
	}
	if err := httpsURL(r.URL); err != nil {
		return err
	}
	digest, err := hex.DecodeString(r.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return errors.New("SHA256 không hợp lệ")
	}
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("Public key không hợp lệ")
	}
	signature, err := base64.StdEncoding.DecodeString(r.Signature)
	if err != nil {
		return err
	}
	message := r.Version + "\n" + r.URL + "\n" + strings.ToLower(r.SHA256)
	if !ed25519.Verify(key, []byte(message), signature) {
		return errors.New("Chữ ký bản cập nhật không hợp lệ")
	}
	return nil
}
func Check(ctx context.Context, channel Channel) (Release, error) {
	var r Release
	if err := ValidateChannel(channel); err != nil {
		return r, err
	}
	if channel.URL == "" {
		return r, errors.New("Chưa cấu hình kênh phát hành")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", channel.URL, nil)
	if err != nil {
		return r, err
	}
	resp, err := client().Do(req)
	if err != nil {
		return r, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return r, fmt.Errorf("Update HTTP %d", resp.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&r); err != nil {
		return r, err
	}
	return r, Verify(r, channel.PublicKey)
}
func Download(ctx context.Context, channel Channel, dir string) (string, error) {
	r, err := Check(ctx, channel)
	if err != nil {
		return "", err
	}
	if !Newer(r.Version, Version) {
		return "", errors.New("Phiên bản trên kênh không mới hơn phiên bản hiện tại")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", r.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Download HTTP %d", resp.StatusCode)
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(dir, "update-*.part")
	if err != nil {
		return "", err
	}
	defer os.Remove(temp.Name())
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(temp, h), io.LimitReader(resp.Body, (512<<20)+1))
	closeErr := temp.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if n > 512<<20 {
		return "", errors.New("Bản cập nhật vượt 512 MB")
	}
	if hex.EncodeToString(h.Sum(nil)) != strings.ToLower(r.SHA256) {
		return "", errors.New("Hash của installer không khớp; tệp đã bị loại bỏ")
	}
	path := filepath.Join(dir, "ConvertFix-"+r.Version+"-setup.exe")
	if err = os.Rename(temp.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

func Newer(candidate, current string) bool {
	left, right := strings.Split(candidate, "."), strings.Split(current, ".")
	if len(left) != 3 || len(right) != 3 {
		return false
	}
	for i := 0; i < 3; i++ {
		a, err := strconv.Atoi(left[i])
		if err != nil {
			return false
		}
		b, err := strconv.Atoi(right[i])
		if err != nil {
			return false
		}
		if a != b {
			return a > b
		}
	}
	return false
}
