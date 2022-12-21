// maestro api
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package login_test

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	pgmocks "github.com/topfreegames/extensions/v9/pg/mocks"
	"github.com/topfreegames/maestro/login/mocks"
	"golang.org/x/oauth2"

	"testing"
)

var (
	mockCtrl   *gomock.Controller
	mockDb     *pgmocks.MockDB
	mockOauth  *mocks.MockGoogleOauthConfig
	mockClient *mocks.MockClient
)

func TestLogin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Login Suite")
}

var _ = BeforeEach(func() {
	mockCtrl = gomock.NewController(GinkgoT())
	mockDb = pgmocks.NewMockDB(mockCtrl)
	mockOauth = mocks.NewMockGoogleOauthConfig(mockCtrl)
	mockClient = mocks.NewMockClient(mockCtrl)
})

//MyTokenSource implements TokenSource
type MyTokenSource struct {
	MyToken *oauth2.Token
	Err     error
}

func (m *MyTokenSource) Token() (*oauth2.Token, error) {
	return m.MyToken, m.Err
}
