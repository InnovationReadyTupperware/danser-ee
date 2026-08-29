package osuapi

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/wieku/danser-go/app/settings"
	"golang.org/x/oauth2"
)

func getToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  settings.Credentails.AccessToken,
		TokenType:    "Bearer",
		RefreshToken: settings.Credentails.RefreshToken,
		Expiry:       settings.Credentails.Expiry,
		ExpiresIn:    int64(settings.Credentails.Expiry.Sub(time.Now()).Seconds()),
	}
}

func getTokenSource() (oauth2.TokenSource, error) {
	return getTokenSourceContext(context.Background())
}

func getTokenSourceContext(ctx context.Context) (oauth2.TokenSource, error) {
	if ctx == nil {
		return nil, errors.New("nil context")
	}
	prepareConfig()

	token := getToken()

	if settings.Credentails.AuthType == "ClientCredentials" {
		if !token.Valid() {
			var err error

			token, err = exchangeClientCredentials(clientConfig, ctx)

			if err != nil {
				return nil, err
			}
		}

		return oauth2.StaticTokenSource(token), nil
	}

	return clientConfig.TokenSource(ctx, token), nil
}

func TryRefreshToken() error {
	return TryRefreshTokenContext(context.Background())
}

// TryRefreshTokenContext refreshes the current token without outliving its
// caller. The launcher uses this form during startup so shutdown can cancel
// the underlying OAuth request before native resources are torn down.
func TryRefreshTokenContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("nil context")
	}
	if settings.Credentails.AccessToken == "" {
		return nil
	}

	tSource, err := getTokenSourceContext(ctx)

	if err != nil {
		return err
	}

	tk, err := tSource.Token()

	if err != nil {
		return err
	}
	if tk == nil {
		return errors.New("token source returned an empty token")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	tryUpdateToken(tk)

	return nil
}

func tryUpdateToken(token *oauth2.Token) {
	if settings.Credentails.AccessToken == token.AccessToken &&
		settings.Credentails.RefreshToken == token.RefreshToken &&
		settings.Credentails.Expiry == token.Expiry {
		return
	}

	settings.Credentails.AccessToken = token.AccessToken
	settings.Credentails.RefreshToken = token.RefreshToken
	settings.Credentails.Expiry = token.Expiry

	if err := settings.SaveCredentialsChecked(false); err != nil {
		log.Println("ApiConnector: Failed to save refreshed credentials:", err)
	}
}
