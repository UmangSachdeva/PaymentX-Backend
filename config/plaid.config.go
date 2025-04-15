package config

import (
	"fmt"
	"os"

	"github.com/plaid/plaid-go/v32/plaid"
)

func PlaidInit() *plaid.APIClient {
	// Initialize the Plaid client
	fmt.Println(os.Getenv("PLAID_CLIENT_ID"))
	fmt.Println(os.Getenv("PLAID_SECRET"))
	configuration := plaid.NewConfiguration()
	configuration.AddDefaultHeader("PLAID-CLIENT-ID", os.Getenv("PLAID_CLIENT_ID"))
	configuration.AddDefaultHeader("PLAID-SECRET", os.Getenv("PLAID_SECRET"))
	configuration.UseEnvironment(plaid.Sandbox)
	client := plaid.NewAPIClient(configuration)

	return client
}
