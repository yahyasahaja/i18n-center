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

// GCSService uploads CMS images to Google Cloud Storage and returns PixelShift URLs for
// serving them through LapakGaming's image CDN pipeline.
//
// # Image serving architecture
//
// LapakGaming uses a double-CDN pipeline to serve images cost-effectively:
//
//	Browser
//	  → Cloudflare (outer CDN, 7-30 day edge cache)
//	      ↓ miss
//	  → PixelShift Cloud Run (on-the-fly resize / format conversion)
//	      ↓ fetches source image
//	  → HAProxy origin shield (caches raw source images)
//	      ↓ miss
//	  → GCS (only on first request for that source image)
//
// PixelShift's API is a simple query-parameter proxy:
//
//	https://img.lapakgaming.com/?src={url-encoded-source-url}&w=720&f=webp&q=75&onerror=redirect
//
// The `src` parameter MUST be a URL reachable through HAProxy (e.g. https://www.lapakgaming.com/static/...)
// and NOT a direct storage.googleapis.com URL.  Reasons:
//
//  1. HAProxy acts as an origin shield between PixelShift and GCS — it caches source images so
//     repeated transforms of the same image don't each hit GCS and pay egress ($0.12/GB).
//  2. Direct GCS URLs bypass this shield entirely, defeating the cost-optimisation architecture.
//  3. HAProxy's CDN config already routes /static/* → correct GCS bucket, so the path mapping
//     is handled transparently without any extra credentials.
//
// # GCS object path
//
// Objects are stored at {GCS_CMS_IMAGE_PREFIX}/{filename} inside {GCS_BUCKET}.
// The default prefix is "static/cms", which HAProxy maps to the lapakgaming-frontend bucket's
// /static/ path — the same routing used for product banners and other platform assets.
//
// # Returned URL
//
// The Upload method returns a PixelShift URL with no transform params:
//
//	https://img.lapakgaming.com/?src=https%3A%2F%2Fwww.lapakgaming.com%2Fstatic%2Fcms%2F{uuid}.jpg
//
// Consumers (FE rendering rich-text HTML) may append transform params as needed, e.g. &w=720&f=webp.
// The Cloudflare cache key includes all query params, so each transform variant is cached separately.
type GCSService struct {
	bucket         string
	pathPrefix     string
	// publicBase is the HAProxy-fronted base URL PixelShift uses to fetch source images.
	// Must be www.lapakgaming.com (or the env-equivalent), never a direct GCS URL.
	publicBase     string
	pixelshiftBase string
	serviceAccount *gcsServiceAccount
	httpClient     *http.Client
}

type gcsServiceAccount struct {
	ClientEmail  string `json:"client_email"`
	PrivateKeyID string `json:"private_key_id"`
	PrivateKey   string `json:"private_key"`
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

	// GCS_CMS_IMAGE_PREFIX is the object path prefix inside the bucket.
	// Default "static/cms" aligns with HAProxy's /static/* → GCS routing rule, which is the
	// same path family used for product banners (e.g. /static/banner/...).
	pathPrefix := os.Getenv("GCS_CMS_IMAGE_PREFIX")
	if pathPrefix == "" {
		pathPrefix = "static/cms"
	}

	// CMS_IMAGE_PUBLIC_BASE is the HAProxy-fronted base URL that becomes the `src` param in
	// PixelShift requests.  This must route through HAProxy so PixelShift's CDN origin shield
	// can cache source images — never use a direct storage.googleapis.com URL here.
	//   dev:  https://dev.lapakgaming.com  (or equivalent staging domain)
	//   prod: https://www.lapakgaming.com
	publicBase := os.Getenv("CMS_IMAGE_PUBLIC_BASE")
	if publicBase == "" {
		publicBase = "https://www.lapakgaming.com"
	}

	// PIXELSHIFT_BASE_URL is the PixelShift service endpoint.
	//   dev:  https://dev-img.lapakgaming.com  (flat subdomain — CF Business plan limitation)
	//   prod: https://img.lapakgaming.com
	pixelshiftBase := os.Getenv("PIXELSHIFT_BASE_URL")
	if pixelshiftBase == "" {
		pixelshiftBase = "https://img.lapakgaming.com"
	}

	return &GCSService{
		bucket:         bucket,
		pathPrefix:     strings.Trim(pathPrefix, "/"),
		publicBase:     strings.TrimSuffix(publicBase, "/"),
		pixelshiftBase: strings.TrimSuffix(pixelshiftBase, "/"),
		serviceAccount: &sa,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Upload stores data in GCS and returns a PixelShift URL for the uploaded image.
//
// The returned URL has the form:
//
//	https://img.lapakgaming.com/?src=https%3A%2F%2Fwww.lapakgaming.com%2Fstatic%2Fcms%2F{uuid}.jpg
//
// This URL is safe to embed directly in rich-text HTML <img> tags. Consumers that need a
// specific size or format can append PixelShift transform params, for example:
//
//	…&w=720&h=400&f=webp&q=75&onerror=redirect
//
// Each unique combination of src + params is cached separately by Cloudflare (7-30 day TTL).
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

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GCS upload request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GCS upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Construct the HAProxy-fronted source URL.
	// This is what PixelShift will fetch when a browser requests the image.
	// HAProxy's CDN origin shield sits between PixelShift and GCS, caching the raw source
	// image so that multiple transform variants (different w/h/f/q params) don't each incur
	// a separate GCS egress hit.
	srcURL := s.publicBase + "/" + objectName

	// Build the final PixelShift URL.  No transform params are added here — the stored URL
	// is the "canonical" reference; FE rendering code appends params as needed per context.
	pixelshiftURL := s.pixelshiftBase + "/?src=" + url.QueryEscape(srcURL)
	return pixelshiftURL, nil
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
