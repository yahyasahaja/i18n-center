package i18ncenter_test

import (
	"fmt"
	"log"
	"os"

	"github.com/your-org/i18n-center-go"
)

func ExampleClient_GetTranslation() {
	// Initialize client
	client := i18ncenter.NewClient(i18ncenter.Config{
		APIBaseURL:  os.Getenv("I18N_CENTER_API_URL"),
		APIToken:    os.Getenv("I18N_CENTER_API_TOKEN"),
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

func ExampleTranslator_T() {
	// Initialize client
	client := i18ncenter.NewClient(i18ncenter.Config{
		APIBaseURL:  os.Getenv("I18N_CENTER_API_URL"),
		APIToken:    os.Getenv("I18N_CENTER_API_TOKEN"),
	})

	// Create translator for a component (application code is required)
	translator := i18ncenter.NewTranslator(client, "my_app", "pdp_form", "en", i18ncenter.StageProduction)

	// Translate a path
	label, err := translator.T("form.name.label")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Label: %s\n", label)
}

func ExampleTranslator_Tf() {
	// Initialize client
	client := i18ncenter.NewClient(i18ncenter.Config{
		APIBaseURL:  os.Getenv("I18N_CENTER_API_URL"),
		APIToken:    os.Getenv("I18N_CENTER_API_TOKEN"),
	})

	// Create translator (application code is required)
	translator := i18ncenter.NewTranslator(client, "my_app", "pdp_form", "en", i18ncenter.StageProduction)

	// Translate with template variables
	greeting, err := translator.Tf("greeting", map[string]interface{}{
		"name": "John",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Greeting: %s\n", greeting)
	// Output: Greeting: Hello John! (if translation is "Hello {name}!")
}

func ExampleSyncTranslator_T() {
	// Assume you already have translation data (e.g., from getServerSideProps equivalent)
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
	fmt.Printf("Label: %s\n", label)
	// Output: Label: Product Name
}

func ExampleClient_GetMultipleTranslations() {
	// Initialize client
	client := i18ncenter.NewClient(i18ncenter.Config{
		APIBaseURL:  os.Getenv("I18N_CENTER_API_URL"),
		APIToken:    os.Getenv("I18N_CENTER_API_TOKEN"),
	})

	// Get multiple translations at once (application code is required)
	translations, err := client.GetMultipleTranslations(
		"my_app", // Application code
		[]string{"pdp_form", "checkout", "cart"},
		"en",
		i18ncenter.StageProduction,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Use translations
	pdpForm := translations["pdp_form"]
	checkout := translations["checkout"]
	cart := translations["cart"]

	fmt.Printf("PDP Form: %+v\n", pdpForm)
	fmt.Printf("Checkout: %+v\n", checkout)
	fmt.Printf("Cart: %+v\n", cart)
}

