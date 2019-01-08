package credhub_test

import (
	"net/http/httptest"
	"testing"

	credhub "github.com/cloudfoundry-community/go-credhub"

	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestDeleteCredentials(t *testing.T) {
	spec.Run(t, "DeleteCredentials", testDeleteCredentials, spec.Report(report.Terminal{}))
}

func testDeleteCredentials(t *testing.T, when spec.G, it spec.S) {
	var (
		server   *httptest.Server
		chClient *credhub.Client
	)

	it.Before(func() {
		RegisterTestingT(t)
		server = mockCredhubServer()
		chClient = credhub.New(server.URL, getAuthenticatedClient(server.Client()))
	})

	it.After(func() {
		server.Close()
	})

	when("testing delete credentials", func() {
		when("it can find the credential", func() {
			it("works", func() {
				err := chClient.Delete("/some-cred")
				Expect(err).To(Not(HaveOccurred()))
			})
		})

		when("it cannot find the credential", func() {
			it("fails", func() {
				err := chClient.Delete("/some-other-cred")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("expected return code 204, got 404"))
			})
		})
	})
}
