package dockerauth

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/util"
)

type AuthHelperSuite struct {
	*util.TestSuite
}

func (a *AuthHelperSuite) TestNormalizeRegistry() {
	quay := "https://quay.io/v1/"
	dockv1 := "https://index.docker.io/v1/"
	dockv2 := "https://index.docker.io/v2/"
	a.Equal(quay, NormalizeRegistry("https://quay.io"))
	a.Equal(quay, NormalizeRegistry("https://quay.io/v1"))
	a.Equal(quay, NormalizeRegistry("http://quay.io/v1"))
	a.Equal(quay, NormalizeRegistry("https://quay.io/v1/"))
	a.Equal(quay, NormalizeRegistry("quay.io"))

	a.Equal(dockv2, NormalizeRegistry(""))
	a.Equal(dockv1, NormalizeRegistry("https://index.docker.io"))
	a.Equal(dockv1, NormalizeRegistry("http://index.docker.io"))
	a.Equal(dockv1, NormalizeRegistry("index.docker.io"))
	a.Equal("https://quay.io/v2/", NormalizeRegistry("quay.io/v2/"))
}

func TestExampleTestSuite(t *testing.T) {
	suiteTester := &AuthHelperSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}
