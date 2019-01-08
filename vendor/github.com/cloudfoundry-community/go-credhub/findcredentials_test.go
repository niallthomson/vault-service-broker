package credhub_test

import (
	"net/http/httptest"
	"testing"

	credhub "github.com/cloudfoundry-community/go-credhub"

	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestFindCredentials(t *testing.T) {
	spec.Run(t, "FindCredentials", testFindCredentials, spec.Report(report.Terminal{}))
}

func testFindCredentials(t *testing.T, when spec.G, it spec.S) {
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

	when("Testing Find By Path", func() {
		it("should be able to find creds by path", func() {
			creds, err := chClient.FindByPath("/concourse/common")
			Expect(err).To(BeNil())
			Expect(len(creds)).To(Equal(3))
		})

		it("should not be able to find creds with an unknown path", func() {
			creds, err := chClient.FindByPath("/concourse/uncommon")
			Expect(err).To(HaveOccurred())
			Expect(len(creds)).To(Equal(0))
		})
	})

	when("Testing List All Paths", func() {
		it("should list all paths", func() {
			paths, err := chClient.ListAllPaths()
			Expect(err).To(Not(HaveOccurred()))
			Expect(paths).To(HaveLen(5))
		})
	})

	when("Testing Find By Name", func() {
		it("should return names with 'password' in them", func() {
			creds, err := chClient.FindByPartialName("password")
			Expect(err).To(Not(HaveOccurred()))
			Expect(creds).To(HaveLen(2))
			for _, cred := range creds {
				Expect(cred.Name).To(ContainSubstring("password"))
			}
		})
	})
}
