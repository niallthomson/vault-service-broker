package credhub_test

import (
	"net/http/httptest"
	"os"
	"testing"

	credhub "github.com/cloudfoundry-community/go-credhub"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestModifyPermissions(t *testing.T) {
	spec.Run(t, "ModifyPermissions", testModifyPermissions, spec.Report(report.Terminal{}))
}

func testModifyPermissions(t *testing.T, when spec.G, it spec.S) {
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
		err := os.Remove("testdata/permissions/add-permissions/cred.json")
		Expect(err).NotTo(HaveOccurred())
		server.Close()
	})

	when("Modifying Permissions", func() {
		it("Works", func() {
			var perms []credhub.Permission
			var err error

			perms, err = chClient.GetPermissions("/add-permission-credential")
			Expect(err).NotTo(HaveOccurred())
			Expect(perms).To(HaveLen(0))

			perms = append(perms, credhub.Permission{
				Actor:      "uaa-user:1234",
				Operations: []credhub.Operation{"read", "write", "delete"},
			})

			respPerms, err := chClient.AddPermissions("/add-permission-credential", perms)
			Expect(err).NotTo(HaveOccurred())
			Expect(respPerms).To(HaveLen(1))
			Expect(respPerms[0].Actor).To(Equal("uaa-user:1234"))

			err = chClient.DeletePermissions("/add-permission-credential", "some-non-existent-actor")
			Expect(err).NotTo(HaveOccurred())
			perms, err = chClient.GetPermissions("/add-permission-credential")
			Expect(err).NotTo(HaveOccurred())
			Expect(perms).To(HaveLen(1))
			Expect(perms[0].Actor).To(Equal("uaa-user:1234"))

			err = chClient.DeletePermissions("/add-permission-credential", "uaa-user:1234")
			Expect(err).NotTo(HaveOccurred())
			perms, err = chClient.GetPermissions("/add-permission-credential")
			Expect(err).NotTo(HaveOccurred())
			Expect(perms).To(HaveLen(0))
		})
	})
}
