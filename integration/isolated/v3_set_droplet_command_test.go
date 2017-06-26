package isolated

import (
	"os"
	"regexp"

	"code.cloudfoundry.org/cli/integration/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("v3-set-droplet command", func() {
	var (
		orgName   string
		spaceName string
		appName   string
	)

	BeforeEach(func() {
		Skip("don't run in the pipeline until cf-deployment master supports it")

		orgName = helpers.NewOrgName()
		spaceName = helpers.NewSpaceName()
		appName = helpers.PrefixedRandomName("app")
	})

	Describe("help", func() {
		Context("when --help flag is set", func() {
			It("Displays command usage to output", func() {
				session := helpers.CF("v3-set-droplet", "--help")

				Eventually(session.Out).Should(Say("NAME:"))
				Eventually(session.Out).Should(Say("v3-set-droplet - Set the droplet used to run an app"))
				Eventually(session.Out).Should(Say("USAGE:"))
				Eventually(session.Out).Should(Say("cf v3-set-droplet -n APP_NAME -d DROPLET_GUID"))
				Eventually(session.Out).Should(Say("OPTIONS:"))
				Eventually(session.Out).Should(Say("--name, -n\\s+The application name to which to assign the droplet"))
				Eventually(session.Out).Should(Say("--droplet-guid, -d\\s+The guid of the droplet to use"))

				Eventually(session).Should(Exit(0))
			})
		})
	})

	Context("when no flags are given", func() {
		It("displays incorrect usage", func() {
			session := helpers.CF("v3-set-droplet")

			Eventually(session.Err).Should(Say("Incorrect Usage: the required flags `-d, --droplet-guid' and `-n, --name' were not specified"))
			Eventually(session.Out).Should(Say("NAME:"))

			Eventually(session).Should(Exit(1))
		})
	})

	Context("when the name flag is missing", func() {
		It("displays incorrect usage", func() {
			session := helpers.CF("v3-set-droplet", "--droplet-guid", "some-droplet-guid")

			Eventually(session.Err).Should(Say("Incorrect Usage: the required flag `-n, --name' was not specified"))
			Eventually(session.Out).Should(Say("NAME:"))

			Eventually(session).Should(Exit(1))
		})
	})

	Context("when the package GUID flag is missing", func() {
		It("displays incorrect usage", func() {
			session := helpers.CF("v3-set-droplet", "--name", "some-app")

			Eventually(session.Err).Should(Say("Incorrect Usage: the required flag `-d, --droplet-guid' was not specified"))
			Eventually(session.Out).Should(Say("NAME:"))

			Eventually(session).Should(Exit(1))
		})
	})

	Context("when the environment is not setup correctly", func() {
		Context("when no API endpoint is set", func() {
			BeforeEach(func() {
				helpers.UnsetAPI()
			})

			It("fails with no API endpoint set message", func() {
				session := helpers.CF("v3-set-droplet", "--name", appName, "--droplet-guid", "some-droplet-guid")
				Eventually(session).Should(Say("FAILED"))
				Eventually(session.Err).Should(Say("No API endpoint set\\. Use 'cf login' or 'cf api' to target an endpoint\\."))
				Eventually(session).Should(Exit(1))
			})
		})

		Context("when not logged in", func() {
			BeforeEach(func() {
				helpers.LogoutCF()
			})

			It("fails with not logged in message", func() {
				session := helpers.CF("v3-set-droplet", "--name", appName, "--droplet-guid", "some-droplet-guid")
				Eventually(session).Should(Say("FAILED"))
				Eventually(session.Err).Should(Say("Not logged in\\. Use 'cf login' to log in\\."))
				Eventually(session).Should(Exit(1))
			})
		})

		Context("when there is no org set", func() {
			BeforeEach(func() {
				helpers.LogoutCF()
				helpers.LoginCF()
			})

			It("fails with no org targeted error message", func() {
				session := helpers.CF("v3-set-droplet", "--name", appName, "--droplet-guid", "some-droplet-guid")
				Eventually(session.Out).Should(Say("FAILED"))
				Eventually(session.Err).Should(Say("No org targeted, use 'cf target -o ORG' to target an org\\."))
				Eventually(session).Should(Exit(1))
			})
		})

		Context("when there is no space set", func() {
			BeforeEach(func() {
				helpers.LogoutCF()
				helpers.LoginCF()
				helpers.TargetOrg(ReadOnlyOrg)
			})

			It("fails with no space targeted error message", func() {
				session := helpers.CF("v3-set-droplet", "--name", appName, "--droplet-guid", "some-droplet-guid")
				Eventually(session.Out).Should(Say("FAILED"))
				Eventually(session.Err).Should(Say("No space targeted, use 'cf target -s SPACE' to target a space\\."))
				Eventually(session).Should(Exit(1))
			})
		})
	})

	Context("when the environment is set up correctly", func() {
		BeforeEach(func() {
			setupCF(orgName, spaceName)
		})

		Context("when the app exists", func() {
			var dropletGUID string

			BeforeEach(func() {
				var packageGUID string
				Eventually(helpers.CF("v3-create-app", "--name", appName)).Should(Exit(0))

				prevDir, err := os.Getwd()
				Expect(err).ToNot(HaveOccurred())

				helpers.WithHelloWorldApp(func(appDir string) {
					err := os.Chdir(appDir)
					Expect(err).ToNot(HaveOccurred())

					pkgSession := helpers.CF("v3-create-package", "--name", appName)
					Eventually(pkgSession).Should(Exit(0))
					regex, err := regexp.Compile(`package guid: (.+)`)
					Expect(err).ToNot(HaveOccurred())
					matches := regex.FindStringSubmatch(string(pkgSession.Out.Contents()))
					Expect(matches).To(HaveLen(2))

					packageGUID = matches[1]
				})

				err = os.Chdir(prevDir)
				Expect(err).ToNot(HaveOccurred())

				stageSession := helpers.CF("v3-stage", "--name", appName, "--package-guid", packageGUID)
				Eventually(stageSession).Should(Exit(0))

				regex, err := regexp.Compile(`droplet: (.+)`)
				Expect(err).ToNot(HaveOccurred())
				matches := regex.FindStringSubmatch(string(stageSession.Out.Contents()))
				Expect(matches).To(HaveLen(2))

				dropletGUID = matches[1]
			})

			It("sets the droplet for the app", func() {
				userName, _ := helpers.GetCredentials()

				session := helpers.CF("v3-set-droplet", "-n", appName, "-d", dropletGUID)
				Eventually(session.Out).Should(Say("Setting app %s to droplet %s in org %s / space %s as %s\\.\\.\\.", appName, dropletGUID, orgName, spaceName, userName))
				Eventually(session.Out).Should(Say("OK"))

				Eventually(session).Should(Exit(0))
			})

			Context("when the app does not exist", func() {
				It("displays app not found and exits 1", func() {
					invalidAppName := "invalid-app-name"
					session := helpers.CF("v3-set-droplet", "-n", invalidAppName, "-d", dropletGUID)
					userName, _ := helpers.GetCredentials()

					Eventually(session.Out).Should(Say("Setting app %s to droplet %s in org %s / space %s as %s\\.\\.\\.", invalidAppName, dropletGUID, orgName, spaceName, userName))
					Eventually(session.Err).Should(Say("App %s not found", invalidAppName))
					Eventually(session.Out).Should(Say("FAILED"))

					Eventually(session).Should(Exit(1))
				})
			})

			Context("when the droplet does not exist", func() {
				It("displays droplet not found and exits 1", func() {
					invalidDropletGUID := "some-droplet-guid"
					session := helpers.CF("v3-set-droplet", "-n", appName, "-d", invalidDropletGUID)
					userName, _ := helpers.GetCredentials()

					Eventually(session.Out).Should(Say("Setting app %s to droplet %s in org %s / space %s as %s\\.\\.\\.", appName, invalidDropletGUID, orgName, spaceName, userName))
					Eventually(session.Err).Should(Say("Unable to assign current droplet\\. Ensure the droplet exists and belongs to this app\\."))
					Eventually(session.Out).Should(Say("FAILED"))
					Eventually(session).Should(Exit(1))
				})
			})
		})
	})
})