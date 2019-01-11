package credhub_test

import (
	"net/http/httptest"
	"testing"

	credhub "github.com/cloudfoundry-community/go-credhub"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestGetPermissions(t *testing.T) {
	spec.Run(t, "GetPermissions", testGetPermissions, spec.Report(report.Terminal{}))
}

func testGetPermissions(t *testing.T, when spec.G, it spec.S) {
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

	when("getting permissions", func() {
		it("works", func() {
			perms, err := chClient.GetPermissions("/credential-with-permissions")
			Expect(err).NotTo(HaveOccurred())
			Expect(perms).To(HaveLen(3))

			perms, err = chClient.GetPermissions("/non-existent")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(BeEquivalentTo("credential not found"))
			Expect(perms).To(BeNil())
		})
	})
}
