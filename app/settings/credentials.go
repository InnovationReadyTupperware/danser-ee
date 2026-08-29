package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/wieku/danser-go/framework/env"
	"github.com/wieku/danser-go/framework/files"
)

var Credentails = &credentials{
	AuthType:     "ClientCredentials",
	CallbackPort: 8294,
}

type credentials struct {
	ClientId     string
	ClientSecret string `long:"true" password:"true"`

	AuthType string `combo:"ClientCredentials|Client credentials (Anonymous),AuthorizationCode|Authorization code (User authenticated)"`
	//
	CallbackPort int `string:"true" min:"0" max:"65535" showif:"AuthType=AuthorizationCode"`

	AccessToken  string    `skip:"true" long:"true" password:"true"`
	Expiry       time.Time `skip:"true"`
	RefreshToken string    `skip:"true" long:"true" password:"true" showif:"AuthType=AuthorizationCode"`
}

var srcDataCred []byte

func LoadCredentials() {
	if err := os.MkdirAll(env.ConfigDir(), 0755); err != nil {
		panic(err)
	}

	file, err := os.Open(filepath.Join(env.ConfigDir(), "credentials.json"))

	if os.IsNotExist(err) {
		SaveCredentials(true)
	} else if err != nil {
		panic(err)
	} else {
		defer file.Close()

		loadCredentials(file)

		SaveCredentials(false) // this is done to save additions from the current format
	}
}

func loadCredentials(file *os.File) {
	log.Println(fmt.Sprintf(`ApiConnector: Loading "%s"`, file.Name()))

	data, err := io.ReadAll(files.NewUnicodeReader(file))
	if err != nil {
		panic(err)
	}

	srcDataCred = data

	if err = json.Unmarshal(data, Credentails); err != nil {
		panic(fmt.Sprintf("ApiConnector: Failed to parse %s! Please re-check the file for mistakes. Error: %s", file.Name(), err))
	}
}

func SaveCredentials(forceSave bool) {
	if err := SaveCredentialsChecked(forceSave); err != nil {
		panic(err)
	}
}

// SaveCredentialsChecked persists credentials without turning an ordinary
// filesystem failure into a process panic. Callers that can surface or log a
// persistence error should prefer this form; SaveCredentials remains as the
// compatibility wrapper for older startup paths.
func SaveCredentialsChecked(forceSave bool) error {
	data, err := json.MarshalIndent(Credentails, "", "\t")
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}

	fPath := filepath.Join(env.ConfigDir(), "credentials.json")

	if forceSave || !bytes.Equal(data, srcDataCred) { // Don't rewrite the file unless necessary
		log.Println(fmt.Sprintf(`ApiConnector: Saving current settings to "%s"`, fPath))

		if err = os.MkdirAll(filepath.Dir(fPath), 0755); err != nil {
			return fmt.Errorf("create credentials directory: %w", err)
		}

		if err = os.WriteFile(fPath, data, 0644); err != nil {
			return fmt.Errorf("write credentials: %w", err)
		}

		srcDataCred = data
	}

	return nil
}
