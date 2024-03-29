package e2e

import (
	"fmt"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/rosa/tests/ci/labels"
	"github.com/openshift/rosa/tests/utils/common"
	"github.com/openshift/rosa/tests/utils/config"
	"github.com/openshift/rosa/tests/utils/exec/rosacli"
)

var _ = Describe("Edit OIDC config",
	labels.Day2,
	labels.FeatureOidcConfig,
	func() {
		defer GinkgoRecover()

		var (
			clusterID                string
			oidcConfigIDsNeedToClean []string
			installerRoleArn         string
			hostedCP                 bool
			err                      error

			rosaClient         *rosacli.Client
			ocmResourceService rosacli.OCMResourceService
		)

		BeforeEach(func() {
			By("Get the cluster ID")
			clusterID = config.GetClusterID()
			Expect(clusterID).ToNot(Equal(""), "ClusterID is required. Please export CLUSTER_ID")

			By("Init the client")
			rosaClient = rosacli.NewClient()
			ocmResourceService = rosaClient.OCMResource

			By("Get if hosted")
			hostedCP, err = rosaClient.Cluster.IsHostedCPCluster(clusterID)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			By("Clean remaining resources")
			err := rosaClient.CleanResources(clusterID)
			Expect(err).ToNot(HaveOccurred())
		})

		It("can create/list/delete BYO oidc config in auto mode - [id:57570]",
			labels.High,
			labels.MigrationToVerify,
			labels.Exclude,
			func() {
				defer func() {
					By("make sure that all oidc configs created during the testing")
					if len(oidcConfigIDsNeedToClean) > 0 {
						By("Delete oidc configs")
						for _, id := range oidcConfigIDsNeedToClean {
							output, err := ocmResourceService.DeleteOIDCConfig(
								"--oidc-config-id", id,
								"--mode", "auto",
								"-y",
							)
							Expect(err).To(BeNil())
							textData := rosaClient.Parser.TextData.Input(output).Parse().Tip()
							Expect(textData).To(ContainSubstring("Successfully deleted the OIDC provider"))

							By("Check the managed oidc config is deleted")
							oidcConfigList, _, err := ocmResourceService.ListOIDCConfig()
							Expect(err).To(BeNil())
							foundOIDCConfig := oidcConfigList.OIDCConfig(id)
							Expect(foundOIDCConfig).To(Equal(rosacli.OIDCConfig{}))
						}
					}
				}()

				var (
					oidcConfigPrefix       = "op57570"
					longPrefix             = "1234567890abcdef"
					notExistedOODCConfigID = "notexistedoidcconfigid111"
					unmanagedOIDCConfigID  string
					managedOIDCConfigID    string
					accountRolePrefix      string
				)
				By("Create account-roles for testing")
				rand.Seed(time.Now().UnixNano())
				accountRolePrefix = fmt.Sprintf("QEAuto-accr57570-%s", time.Now().UTC().Format("20060102"))
				_, err := ocmResourceService.CreateAccountRole("--mode", "auto",
					"--prefix", accountRolePrefix,
					"-y")
				Expect(err).To(BeNil())

				defer func() {
					By("Cleanup created account-roles")
					_, err := ocmResourceService.DeleteAccountRole("--mode", "auto",
						"--prefix", accountRolePrefix,
						"-y")
					Expect(err).To(BeNil())
				}()

				By("Get the installer role arn")
				accountRoleList, _, err := ocmResourceService.ListAccountRole()
				Expect(err).To(BeNil())
				installerRole := accountRoleList.InstallerRole(accountRolePrefix, hostedCP)
				Expect(installerRole).ToNot(BeNil())
				installerRoleArn = installerRole.RoleArn

				By("Create managed=false oidc config in auto mode")
				output, err := ocmResourceService.CreateOIDCConfig("--mode", "auto",
					"--prefix", oidcConfigPrefix,
					"--installer-role-arn", installerRoleArn,
					"--managed=false",
					"-y")
				Expect(err).To(BeNil())
				textData := rosaClient.Parser.TextData.Input(output).Parse().Tip()
				Expect(textData).To(ContainSubstring("Created OIDC provider with ARN"))

				oidcPrivodeARNFromOutputMessage := common.ExtractOIDCProviderARN(output.String())
				oidcPrivodeIDFromOutputMessage := common.ExtractOIDCProviderIDFromARN(oidcPrivodeARNFromOutputMessage)

				unmanagedOIDCConfigID, err = ocmResourceService.GetOIDCIdFromList(oidcPrivodeIDFromOutputMessage)
				Expect(err).To(BeNil())

				oidcConfigIDsNeedToClean = append(oidcConfigIDsNeedToClean, unmanagedOIDCConfigID)

				By("Check the created unmananged oidc by `rosa list oidc-config`")
				oidcConfigList, output, err := ocmResourceService.ListOIDCConfig()
				Expect(err).To(BeNil())
				foundOIDCConfig := oidcConfigList.OIDCConfig(unmanagedOIDCConfigID)
				Expect(foundOIDCConfig).NotTo(BeNil())
				Expect(foundOIDCConfig.Managed).To(Equal("false"))
				Expect(foundOIDCConfig.SecretArn).NotTo(Equal(""))
				Expect(foundOIDCConfig.ID).To(Equal(unmanagedOIDCConfigID))

				By("Create managed oidc config in auto mode")
				output, err = ocmResourceService.CreateOIDCConfig("--mode", "auto", "-y")
				Expect(err).To(BeNil())
				textData = rosaClient.Parser.TextData.Input(output).Parse().Tip()
				Expect(textData).To(ContainSubstring("Created OIDC provider with ARN"))
				oidcPrivodeARNFromOutputMessage = common.ExtractOIDCProviderARN(output.String())
				oidcPrivodeIDFromOutputMessage = common.ExtractOIDCProviderIDFromARN(oidcPrivodeARNFromOutputMessage)

				managedOIDCConfigID, err = ocmResourceService.GetOIDCIdFromList(oidcPrivodeIDFromOutputMessage)
				Expect(err).To(BeNil())

				oidcConfigIDsNeedToClean = append(oidcConfigIDsNeedToClean, managedOIDCConfigID)

				By("Check the created mananged oidc by `rosa list oidc-config`")
				oidcConfigList, output, err = ocmResourceService.ListOIDCConfig()
				Expect(err).To(BeNil())
				foundOIDCConfig = oidcConfigList.OIDCConfig(managedOIDCConfigID)
				Expect(foundOIDCConfig).NotTo(BeNil())
				Expect(foundOIDCConfig.Managed).To(Equal("true"))
				Expect(foundOIDCConfig.IssuerUrl).To(ContainSubstring(foundOIDCConfig.ID))
				Expect(foundOIDCConfig.SecretArn).To(Equal(""))
				Expect(foundOIDCConfig.ID).To(Equal(managedOIDCConfigID))

				By("Validate the invalid mode")
				output, err = ocmResourceService.CreateOIDCConfig("--mode", "invalidmode", "-y")
				Expect(err).NotTo(BeNil())
				textData = rosaClient.Parser.TextData.Input(output).Parse().Tip()

				Expect(textData).To(ContainSubstring("Invalid mode. Allowed values are [auto manual]"))

				By("Validate the prefix length")
				output, err = ocmResourceService.CreateOIDCConfig(
					"--mode", "auto",
					"--prefix", longPrefix,
					"--managed=false",
					"-y")
				Expect(err).NotTo(BeNil())
				textData = rosaClient.Parser.TextData.Input(output).Parse().Tip()
				Expect(textData).To(ContainSubstring("length of prefix is limited to 15 characters"))

				By("Validate the prefix and managed at the same time")
				output, err = ocmResourceService.CreateOIDCConfig(
					"--mode", "auto",
					"--prefix", oidcConfigPrefix,
					"-y")
				Expect(err).NotTo(BeNil())
				textData = rosaClient.Parser.TextData.Input(output).Parse().Tip()
				Expect(textData).To(ContainSubstring("prefix param is not supported for managed OIDC config"))

				By("Validation the installer-role-arn and managed at the same time")
				output, err = ocmResourceService.CreateOIDCConfig(
					"--mode", "auto",
					"--installer-role-arn", installerRoleArn,
					"-y")
				Expect(err).NotTo(BeNil())
				textData = rosaClient.Parser.TextData.Input(output).Parse().Tip()
				Expect(textData).To(ContainSubstring("role-arn param is not supported for managed OIDC config"))

				By("Validation the raw-files and managed at the same time")
				output, err = ocmResourceService.CreateOIDCConfig(
					"--mode", "auto",
					"--raw-files",
					"-y")
				Expect(err).NotTo(BeNil())
				textData = rosaClient.Parser.TextData.Input(output).Parse().Tip()
				Expect(textData).To(ContainSubstring("--raw-files param is not supported alongside --mode param"))

				By("Validate the oidc-config deletion with no-existed oidc config id in auto mode")
				output, err = ocmResourceService.DeleteOIDCConfig(
					"--mode", "auto",
					"--oidc-config-id", notExistedOODCConfigID,
					"-y")
				Expect(err).NotTo(BeNil())
				textData = rosaClient.Parser.TextData.Input(output).Parse().Tip()
				Expect(textData).To(ContainSubstring("not found"))
			})
	})
