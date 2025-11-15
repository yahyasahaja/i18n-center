# i18n-center-go

Go SDK for the i18n-center translation service. Provides a simple, type-safe interface for fetching and using translations in Go applications.

## Installation

```bash
go get github.com/your-org/i18n-center-go
```

## Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "log"

    "github.com/your-org/i18n-center-go"
)

func main() {
    // Initialize client
    client := i18ncenter.NewClient(i18ncenter.Config{
        APIBaseURL:  "https://api.example.com/api",
        APIToken:    "your-api-token", // Optional
        DefaultLocale: "en",
        DefaultStage:  i18ncenter.StageProduction,
    })

    // Get translation for a component (application code is required)
    translation, err := client.GetTranslation("my_app", "pdp_form", "en", i18ncenter.StageProduction)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Translation: %+v\n", translation)
}
```

### Using Translator (Recommended)

```go
package main

import (
    "fmt"
    "log"

    "github.com/your-org/i18n-center-go"
)

func main() {
    // Initialize client
    client := i18ncenter.NewClient(i18ncenter.Config{
        APIBaseURL: "https://api.example.com/api",
        APIToken:   "your-api-token",
    })

    // Create translator for a component (application code is required)
    translator := i18ncenter.NewTranslator(client, "my_app", "pdp_form", "en", i18ncenter.StageProduction)

    // Translate a path
    label, err := translator.T("form.name.label")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Label: %s\n", label)
    // Output: Label: Product Name
}
```

### Translation with Template Variables

```go
translator := i18ncenter.NewTranslator(client, "pdp_form", "en", i18ncenter.StageProduction)

// Translation: "Hello {name}!"
greeting, err := translator.Tf("greeting", map[string]interface{}{
    "name": "John",
})
// Output: "Hello John!"
```

### Multiple Components

```go
// Get multiple translations at once (efficient, uses bulk API)
// Application code is required to differentiate components with the same code in different applications
translations, err := client.GetMultipleTranslations(
    "my_app", // Application code (required)
    []string{"pdp_form", "checkout", "cart"},
    "en",
    i18ncenter.StageProduction,
)

// Use translations
pdpForm := translations["pdp_form"]
checkout := translations["checkout"]
cart := translations["cart"]
```

### Synchronous Translator (Preloaded Data)

When you already have translation data (e.g., from a previous API call or cache):

```go
// Assume you have translation data
translationData := i18ncenter.TranslationData{
    "form": map[string]interface{}{
        "name": map[string]interface{}{
            "label": "Product Name",
        },
    },
}

// Create synchronous translator (no API calls)
translator := i18ncenter.NewSyncTranslator(translationData)

// Use synchronously
label := translator.T("form.name.label")
// Output: "Product Name"
```

## API Reference

### `Client`

Main client for interacting with the i18n-center API.

```go
client := i18ncenter.NewClient(i18ncenter.Config{
    APIBaseURL:  string,              // Required: API base URL
    APIToken:    string,              // Optional: Bearer token
    DefaultLocale: string,            // Default: "en"
    DefaultStage: DeploymentStage,     // Default: StageProduction
    CacheTTL:    time.Duration,       // Default: 1 hour
    EnableCache: bool,                 // Default: true
    HTTPClient:  *http.Client,        // Optional: Custom HTTP client
})
```

**Methods:**

- `GetTranslation(applicationCode, componentCode, locale, stage)`: Get translation for a single component. **Application code is required** to differentiate components with the same code in different applications.
- `GetMultipleTranslations(applicationCode, componentCodes, locale, stage)`: Get translations for multiple components. **Application code is required**.
- `ClearCache()`: Clear the cache

### `Translator`

Provides translation functions for a specific component. **Application code is required** to differentiate components with the same code in different applications.

```go
translator := i18ncenter.NewTranslator(client, "my_app", "pdp_form", "en", i18ncenter.StageProduction)

// Methods:
label, err := translator.T("form.name.label")
greeting, err := translator.Tf("greeting", map[string]interface{}{"name": "John"})
data, err := translator.GetRaw()
err := translator.Preload()
```

**Translation Path Syntax:**

Use dot notation to access nested translation values:
- `"form.name.label"` → `translation["form"]["name"]["label"]`
- `"button.submit"` → `translation["button"]["submit"]`

**Template Variables:**

Support for `{variable}` and `[variable]` syntax:
```go
// Translation: "Hello {name}!"
translator.Tf("greeting", map[string]interface{}{"name": "John"})
// Returns: "Hello John!"
```

### `SyncTranslator`

Synchronous translator for preloaded data (no API calls).

```go
translator := i18ncenter.NewSyncTranslator(translationData)

// Methods (all synchronous, no errors):
label := translator.T("form.name.label")
greeting := translator.Tf("greeting", map[string]interface{}{"name": "John"})
data := translator.GetRaw()
```

## Configuration

### Environment Variables

```go
client := i18ncenter.NewClient(i18ncenter.Config{
    APIBaseURL:  os.Getenv("I18N_CENTER_API_URL"),
    APIToken:    os.Getenv("I18N_CENTER_API_TOKEN"),
    DefaultLocale: "en",
    DefaultStage:  i18ncenter.StageProduction,
})
```

## Caching

The SDK includes built-in in-memory caching:

- **Default TTL**: 1 hour
- **Cache Key**: `i18n:{componentCode}:{locale}:{stage}`
- **Automatic**: Cache is checked before API calls
- **Manual Clear**: `client.ClearCache()`

Caching is enabled by default. To disable:

```go
client := i18ncenter.NewClient(i18ncenter.Config{
    APIBaseURL:  "...",
    EnableCache: false,
})
```

## Deployment Stages

```go
const (
    StageDraft      DeploymentStage = "draft"
    StageStaging    DeploymentStage = "staging"
    StageProduction DeploymentStage = "production"
)
```

## Error Handling

All methods that make API calls return errors:

```go
translation, err := client.GetTranslation("pdp_form", "en", i18ncenter.StageProduction)
if err != nil {
    // Handle error (network, API error, not found, etc.)
    log.Printf("Error: %v", err)
    return
}
```

## Examples

### HTTP Handler Example

```go
package main

import (
    "encoding/json"
    "net/http"

    "github.com/your-org/i18n-center-go"
)

var client = i18ncenter.NewClient(i18ncenter.Config{
    APIBaseURL: os.Getenv("I18N_CENTER_API_URL"),
    APIToken:   os.Getenv("I18N_CENTER_API_TOKEN"),
})

func productHandler(w http.ResponseWriter, r *http.Request) {
    // Get locale from query or header
    locale := r.URL.Query().Get("locale")
    if locale == "" {
        locale = "en"
    }

    // Create translator (application code is required)
    translator := i18ncenter.NewTranslator(client, "my_app", "pdp_form", locale, i18ncenter.StageProduction)

    // Get translations
    productNameLabel, _ := translator.T("form.name.label")
    addToCartLabel, _ := translator.T("button.add_to_cart")

    response := map[string]string{
        "productNameLabel": productNameLabel,
        "addToCartLabel":   addToCartLabel,
    }

    json.NewEncoder(w).Encode(response)
}
```

### Gin Framework Example

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/your-org/i18n-center-go"
)

var client = i18ncenter.NewClient(i18ncenter.Config{
    APIBaseURL: os.Getenv("I18N_CENTER_API_URL"),
    APIToken:   os.Getenv("I18N_CENTER_API_TOKEN"),
})

func productHandler(c *gin.Context) {
    locale := c.Query("locale")
    if locale == "" {
        locale = "en"
    }

    translator := i18ncenter.NewTranslator(client, "my_app", "pdp_form", locale, i18ncenter.StageProduction)

    label, err := translator.T("form.name.label")
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, gin.H{"label": label})
}
```

### Preloading for Performance

```go
// Preload translations at application startup
func init() {
    translator := i18ncenter.NewTranslator(client, "my_app", "pdp_form", "en", i18ncenter.StageProduction)
    if err := translator.Preload(); err != nil {
        log.Printf("Failed to preload translations: %v", err)
    }
}

// Later, use synchronously (no API calls)
func getLabel() string {
    translator := i18ncenter.NewTranslator(client, "my_app", "pdp_form", "en", i18ncenter.StageProduction)
    label, _ := translator.T("form.name.label")
    return label
}
```

## Type Safety

The SDK uses Go's type system for safety:

- `TranslationData` is `map[string]interface{}` (JSON structure)
- `DeploymentStage` is a typed string constant
- All methods are strongly typed

## License

MIT

