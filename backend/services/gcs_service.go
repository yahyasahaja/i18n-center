package services

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// GCSService handles uploads to Google Cloud Storage using service account credentials.
// Credentials are loaded from GCS_CREDENTIALS_BASE64 env var (base64-encoded service account JSON).
type GCSService struct {
	bucket          string
	pathPrefix      string
	pixelshiftBase  string
	serviceAccount  *gcsServiceAccount
	httpClient      *http.Client
}

type gcsServiceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKeyID string `json:"private_key_id"`
	PrivateKey  string `json:"private_key"`
}

type gcsTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

func NewGCSService() (*GCSService, error) {
	bucket := os.Getenv("GCS_BUCKET")
	if bucket == "" {
		return nil, fmt.Errorf("GCS_BUCKET env var not set")
	}

	credsB64 := os.Getenv("GCS_CREDENTIALS_BASE64")
	if credsB64 == "" {
		return nil, fmt.Errorf("GCS_CREDENTIALS_BASE64 env var not set")
	}

	credsJSON, err := base64.StdEncoding.DecodeString(credsB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode GCS credentials: %w", err)
	}

	var sa gcsServiceAccount
	if err := json.Unmarshal(credsJSON, &sa); err != nil {
		return nil, fmt.Errorf("failed to parse GCS service account: %w", err)
	}
	if sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, fmt.Errorf("GCS service account missing required fields")
	}

	pathPrefix := os.Getenv("GCS_CMS_IMAGE_PREFIX")
	if pathPrefix == "" {
		pathPrefix = "public/cms"
	}
	pixelshiftBase := os.Getenv("PIXELSHIFT_BASE_URL")
	if pixelshiftBase == "" {
		pixelshiftBase = "https://img.lapakgaming.com/s"
	}

	return &GCSService{
		bucket:         bucket,
		pathPrefix:     strings.TrimSuffix(pathPrefix, "/"),
		pixelshiftBase: strings.TrimSuffix(pixelshiftBase, "/"),
		serviceAccount: &sa,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Upload uploads data to GCS and returns the public PixelShift URL.
func (s *GCSService) Upload(ctx context.Context, filename string, contentType string, data []byte) (string, error) {
	token, err := s.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get GCS access token: %w", err)
	}

	objectName := s.pathPrefix + "/" + filename
	uploadURL := fmt.Sprintf(
		"https://www.googleapis.com/upload/storage/v1/b/%s/o?uploadType=media&name=%s",
		url.PathEscape(s.bucket),
		url.QueryEscape(objectName),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(data)))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GCS upload request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GCS upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	// The Cloudflare rewrite strips the GCS bucket prefix:
	// GCS path: {bucket}/{pathPrefix}/{filename}
	// Cloudflare maps /s/* → /lapakgaming-frontend-development/public/* in GCS
	// So PixelShift URL is derived from pathPrefix stripped of the "public/" part.
	publicPath := strings.TrimPrefix(s.pathPrefix, "public/")
	if publicPath == s.pathPrefix {
		// pathPrefix doesn't start with "public/", use as-is
		publicPath = s.pathPrefix
	}
	publicURL := s.pixelshiftBase + "/" + publicPath + "/" + filename

	return publicURL, nil
}

// getAccessToken mints a short-lived OAuth2 access token from the service account private key.
func (s *GCSService) getAccessToken(ctx context.Context) (string, error) {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"iss":   s.serviceAccount.ClientEmail,
		"scope": "https://www.googleapis.com/auth/devstorage.read_write",
		"aud":   "https://oauth2.googleapis.com/token",
		"exp":   now + 3600,
		"iat":   now,
	}

	jwt, err := s.signJWT(claims)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {jwt},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth2.googleapis.com/token",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp gcsTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}
	return tokenResp.AccessToken, nil
}

func (s *GCSService) signJWT(claims map[string]interface{}) (string, error) {
	header := base64.RawURLEncoding.EncodeToString(mustMarshal(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
		"kid": s.serviceAccount.PrivateKeyID,
	}))
	payload := base64.RawURLEncoding.EncodeToString(mustMarshal(claims))
	signingInput := header + "." + payload

	privKey, err := parsePrivateKey(s.serviceAccount.PrivateKey)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	h.Write([]byte(signingInput))
	digest := h.Sum(nil)

	sig, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, digest)
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func parsePrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS1 fallback
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}
	return rsaKey, nil
}

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
