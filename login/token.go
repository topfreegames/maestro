// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package login

import (
	"time"

	"github.com/topfreegames/extensions/v9/pg/interfaces"
	"github.com/topfreegames/maestro/errors"
	"golang.org/x/oauth2"
)

type User struct {
	KeyAccessToken string    `db:"key_access_token"`
	AccessToken    string    `db:"access_token"`
	RefreshToken   string    `db:"refresh_token"`
	Expiry         time.Time `db:"expiry"`
	TokenType      string    `db:"token_type"`
	Email          string    `db:"email"`
}

//SaveToken writes the token parameters on DB
func SaveToken(token *oauth2.Token, email, keyAccessToken string, db interfaces.DB) error {
	query := `INSERT INTO users(key_access_token, access_token, refresh_token, expiry, token_type, email)
	VALUES(?key_access_token, ?access_token, ?refresh_token, ?expiry, ?token_type, ?email)
	ON CONFLICT(email) DO UPDATE
		SET access_token = excluded.access_token,
				key_access_token = users.key_access_token,
				refresh_token = excluded.refresh_token,
				expiry = excluded.expiry`
	if token.RefreshToken == "" {
		query = `UPDATE users
		SET access_token = ?access_token,
				expiry = ?expiry
		WHERE email = ?email`
	}
	user := &User{
		KeyAccessToken: token.AccessToken,
		AccessToken:    token.AccessToken,
		RefreshToken:   token.RefreshToken,
		Expiry:         token.Expiry,
		TokenType:      token.TokenType,
		Email:          email,
	}
	_, err := db.Query(user, query, user)
	if err != nil {
		return errors.NewDatabaseError(err)
	}
	return nil
}

type DestinationToken struct {
	AccessToken  string    `db:"access_token"`
	RefreshToken string    `db:"refresh_token"`
	Expiry       time.Time `db:"expiry"`
	TokenType    string    `db:"token_type"`
}

// GetKeyAccessToken returns key_access_token for a users' email in DB
func GetKeyAccessToken(email string, db interfaces.DB) (string, error) {
	user := &User{}
	query := "SELECT key_access_token FROM users WHERE email = ?"
	_, err := db.Query(user, query, email)
	return user.KeyAccessToken, err
}

//GetToken reads token from DB
func GetToken(accessToken string, db interfaces.DB) (*oauth2.Token, error) {
	query := `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`
	destToken := &DestinationToken{}
	_, err := db.Query(destToken, query, accessToken)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	token := &oauth2.Token{
		AccessToken:  destToken.AccessToken,
		RefreshToken: destToken.RefreshToken,
		Expiry:       destToken.Expiry,
		TokenType:    destToken.TokenType,
	}
	return token, nil
}
